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
"""Handles get requests to get port of grpc channel.

Currently used for titanium runtimes.
"""

import httplib
import os

GRPC_PORT_URL_PATTERN = '_ah/grpc_port'


class Application(object):
  """A simple wsgi app to return the port that grpc is running on."""

  def __call__(self, environ, start_response):
    """Handles WSGI requests.

    Args:
      environ: An environ dict for the current request as defined in PEP-333.
      start_response: A function with semantics defined in PEP-333.

    Returns:
      An environment variable GRPC_PORT.
    """
    start_response('200 %s' % httplib.responses[200], [])
    return os.environ['GRPC_PORT']
