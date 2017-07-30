#!/usr/bin/env python
#
# Copyright 2007 Google Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
"""Provides a logger for logging devappserver2 metrics to Google Analytics.

The MetricsLogger is a singleton class which can be used directly in
devappserver2 scripts or via the few module level helper functions provided
within.

Sample usage in devappserver2:

### In devappserver2.py:

from  google.appengine.tools.devappserver2 import metrics

# When dev_appserver starts, one request is logged to Google Analytics:
metrics_logger = metrics.GetMetricsLogger()
metrics_logger.Start('GoogleAnalyticsClientId', 'UserAgent', {'python27', 'go'})
...
# When dev_appserver stops, a batch request is logged with deferred events.
metrics_logger.Stop()


### In any other devappserver2 libraries:

from  google.appengine.tools.devappserver2 import metrics

# Logging an event immediately:
metrics.GetMetricsLogger().Log('event_category', 'event_action')

# Deferred logging of unique events. These will be logged in batch when
# MetricsLogger.Stop is called. Duplicate events will only be logged once.
metrics.GetMetricsLogger().LogOnceAtStop('event_category', 'event_action')
"""

import datetime
import functools
import httplib
import json
import logging
import sys
import urllib
from google.pyglib import singleton


# Google Analytics Config
_GOOGLE_ANALYTICS_HTTPS_HOST = 'www.google-analytics.com'
_GOOGLE_ANALYTICS_COLLECT_ENDPOINT = '/collect'
_GOOGLE_ANALYTICS_BATCH_ENDPOINT = '/batch'
_GOOGLE_ANALYTICS_VERSION = 1
_GOOGLE_ANALYTICS_TRACKING_ID = 'UA-84862943-2'
_GOOGLE_ANALYTICS_EVENT_TYPE = 'event'

# Devappserver Google Analytics Event Categories
API_STUB_USAGE_CATEGORY = 'api_stub_usage'
DEVAPPSERVER_CATEGORY = 'devappserver'

# Devappserver Google Analytics Event Actions
API_STUB_USAGE_ACTION_TEMPLATE = 'use-%s'
ERROR_ACTION = 'error'
STOP_ACTION = 'stop'
START_ACTION = 'start'

# Devappserver Google Analytics Custom Dimensions.
# This maps the custom dimension name in GAFE to the enumerated cd# parameter
# to be sent with HTTP requests.
_GOOGLE_ANALYTICS_DIMENSIONS = {
    'IsInteractive': 'cd1',
    'Runtimes': 'cd2'
}


class MetricsLoggerError(Exception):
  """Used for MetricsLogger related errors."""


class _MetricsLogger(object):
  """Logs metrics for the devappserver to Google Analytics."""

  def __init__(self):
    """Initializes a _MetricsLogger."""
    self._client_id = None
    self._user_agent = None
    self._runtimes = None
    self._start_time = None
    self._log_once_on_stop_events = {}

  def Start(self, client_id, user_agent=None, runtimes=None):
    """Starts a Google Analytics session for the current client.

    Args:
      client_id: A string Client ID representing a unique anonyized user.
      user_agent: A string user agent to send with each log.
      runtimes: A set of strings containing the runtimes used.
    """
    self._start_time = Now()
    self._client_id = client_id
    self._user_agent = user_agent
    self._runtimes = ','.join(sorted(list(runtimes))) if runtimes else None
    self.Log(DEVAPPSERVER_CATEGORY, START_ACTION)

  def Stop(self):
    """Ends a Google Analytics session for the current client."""
    total_run_time = int((Now() - self._start_time).total_seconds())

    self.LogOnceOnStop(DEVAPPSERVER_CATEGORY, STOP_ACTION, value=total_run_time)
    self.LogBatch(self._log_once_on_stop_events.itervalues())

  def Log(self, category, action, label=None, value=None, **kwargs):
    """Logs a single event to Google Analytics via HTTPS.

    Args:
      category: A string to use as the Google Analytics event category.
      action: A string to use as the Google Analytics event category.
      label: A string to use as the Google Analytics event label.
      value: A number to use as the Google Analytics event value.
      **kwargs: Additional Google Analytics event parameters to include in the
        request body.

    Raises:
      MetricsLoggerError: Raised if the _client_id attribute has not been set
        on the MetricsLogger.
    """
    self._SendRequestToGoogleAnalytics(
        _GOOGLE_ANALYTICS_COLLECT_ENDPOINT,
        self._EncodeEvent(category, action, label, value, **kwargs))

  def LogBatch(self, events):
    """Logs a batch of events to Google Analytics via HTTPS in a single call.

    Args:
      events: An iterable of event dicts whose keys match the args of the
        _EncodeEvent method.
    """
    events = '\n'.join([self._EncodeEvent(**event) for event in events])
    self._SendRequestToGoogleAnalytics(_GOOGLE_ANALYTICS_BATCH_ENDPOINT, events)

  def LogOnceOnStop(self, category, action, label=None, value=None, **kwargs):
    """Stores unique events for deferred batch logging when Stop is called.

    To prevent duplicate events, the raw request parameters are stored in a hash
    table to be batch logged when the Stop method is called.

    Args:
      category: A string to use as the Google Analytics event category.
      action: A string to use as the Google Analytics event category.
      label: A string to use as the Google Analytics event label.
      value: A number to use as the Google Analytics event value.
      **kwargs: Additional Google Analytics event parameters to include in the
        request body.
    """
    request = {
        'category': category,
        'action': action,
        'label': label,
        'value': value,
    }
    request.update(kwargs)
    self._log_once_on_stop_events[json.dumps(request, sort_keys=True)] = request

  def _SendRequestToGoogleAnalytics(self, endpoint, body):
    """Sends an HTTPS POST request to Google Analytics.

    Args:
      endpoint: The string endpoint path for the request, eg "/collect".
      body: The string body to send with the request.

    Raises:
      MetricsLoggerError: Raised if the _client_id attribute has not been set
        on the MetricsLogger.
    """
    if not self._client_id:
      raise MetricsLoggerError(
          'The Client ID must be set to log devappserver metrics.')

    headers = {'User-Agent': self._user_agent} if self._user_agent else {}

    # If anything goes wrong, we do not want to block the main devappserver
    # execution.
    try:
      httplib.HTTPSConnection(_GOOGLE_ANALYTICS_HTTPS_HOST).request(
          'POST', endpoint, body, headers)
    except:  # pylint: disable=bare-except
      logging.debug(
          'Google Analytics request failed: \n %s', str(sys.exc_info()))

  def _EncodeEvent(self, category, action, label=None, value=None, **kwargs):
    """Encodes a single event for sending to Google Analytics.

    Args:
      category: A string to use as the Google Analytics event category.
      action: A string to use as the Google Analytics event category.
      label: A string to use as the Google Analytics event label.
      value: A number to use as the Google Analytics event value.
      **kwargs: Additional Google Analytics event parameters to include in the
        request body.

    Returns:
      A string of the form "key1=value1&key2=value2&key3=value4" containing
      event data and metadata for use in the body of Google Analytics logging
      requests.
    """
    event = {
        # Event metadata
        'v': _GOOGLE_ANALYTICS_VERSION,
        'tid': _GOOGLE_ANALYTICS_TRACKING_ID,
        't': _GOOGLE_ANALYTICS_EVENT_TYPE,
        'cid': self._client_id,
        _GOOGLE_ANALYTICS_DIMENSIONS['IsInteractive']: IsInteractive(),
        _GOOGLE_ANALYTICS_DIMENSIONS['Runtimes']: self._runtimes,

        # Required event data
        'ec': category,
        'ea': action

    }

    # Optional event data
    if label:
      event['el'] = label
    if value:
      event['ev'] = value
    event.update(kwargs)

    return urllib.urlencode(event)


@singleton.Singleton
class MetricsLogger(_MetricsLogger):
  """Singleton MetricsLogger class for logging to Google Analytics."""


# In accordance with Pyglib's Singleton, the first instance is created as
# below, and secondary clients can access the instance via
# MetricsLogger.Singleton(), as in GetMetricsLogger below. We instantiate the
# logger here, so all other uses in devappserver can call GetMetricsLogger.
MetricsLogger()


def GetMetricsLogger():
  """Returns the singleton instance of the MetricsLogger."""
  return MetricsLogger.Singleton()


def GetErrorDetails():
  """Returns a string representation of type and message of an exception."""
  return repr(sys.exc_info()[1])


def IsInteractive():
  """Returns true if the user's session has an interactive stdin."""
  return sys.stdin.isatty()


def Now():
  """Returns a datetime.datetime instance representing the current time.

  This is just a wrapper to ease testing against the datetime module.

  Returns:
    An instance of datetime.datetime.
  """
  return datetime.datetime.now()


class LogHandlerRequest(object):
  """A decorator for logging usage of a webapp2 request handler."""

  def __init__(self, category):
    """Initializes the decorator.

    Args:
      category: The string Google Analytics category for logging requests.
    """
    self._category = category

  def __call__(self, handler_method):
    """Provides a wrapped method for execution.

    Args:
      handler_method: The method that is wrapped by LogHandlerRequest.

    Returns:
      A wrapped handler method.
    """
    @functools.wraps(handler_method)
    def DecoratedHandler(handler_self, *args, **kwargs):
      """Logs the handler_method call and executes the handler_method."""
      GetMetricsLogger().LogOnceOnStop(
          self._category,
          '{class_name}.{method_name}'.format(
              class_name=handler_self.__class__.__name__,
              method_name=handler_method.__name__))
      handler_method(handler_self, *args, **kwargs)

    return DecoratedHandler
