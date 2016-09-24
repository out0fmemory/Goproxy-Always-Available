#!/usr/bin/env python2
# coding:utf-8

import sys
import os
import re
import threading
import base64
import platform

if platform.mac_ver()[0] > '10.':
    sys.exit(os.system(
        'osascript -e \'display dialog "Please click goproxy-macos.command" buttons {"OK"} default button 1 with icon caution with title "GoProxy GTK"\''))

try:
    import pygtk
    pygtk.require('2.0')
    import gtk
    # gtk.gdk.threads_init()
except Exception:
    sys.exit(os.system(
        'gdialog --title "GoProxy GTK" --msgbox "Please install python-gtk2" 15 60'))
try:
    import pynotify
    pynotify.init('GoProxy Notify')
except ImportError:
    pynotify = None
try:
    import appindicator
except ImportError:
    if os.getenv('XDG_CURRENT_DESKTOP', '').lower() == 'unity':
        sys.exit(gtk.MessageDialog(None, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR,
                                   gtk.BUTTONS_OK, u'Please install python-appindicator').run())
    appindicator = None
try:
    import vte
except ImportError:
    sys.exit(gtk.MessageDialog(None, gtk.DIALOG_MODAL, gtk.MESSAGE_ERROR,
                               gtk.BUTTONS_OK, u'Please install python-vte').run())


def spawn_later(seconds, target, *args, **kwargs):
    def wrap(*args, **kwargs):
        import time
        time.sleep(seconds)
        return target(*args, **kwargs)
    t = threading.Thread(target=wrap, args=args, kwargs=kwargs)
    t.setDaemon(True)
    t.run()


def rewrite_desktop(filename):
    with open(filename, 'rb') as fp:
        content = fp.read()
    if 'goproxy-gtk.png' not in content:
        content = re.sub(r'Icon=\S*', 'Icon=%s/goproxy-gtk.png' % os.getcwd(), content)
        with open(filename, 'wb') as fp:
            fp.write(content)

#gtk.main_quit = lambda: None
#appindicator = None


class GoProxyGTK:

    command = [
        os.path.join(os.path.dirname(os.path.abspath(__file__)), 'goproxy'), '-v=2']
    message = u'GoProxy already started.'
    fail_message = u'GoProxy start failed, see the terminal'

    def __init__(self, window, terminal):
        self.window = window
        self.window.set_size_request(652, 447)
        self.window.set_position(gtk.WIN_POS_CENTER)
        self.window.connect('delete-event', self.delete_event)
        self.terminal = terminal

        self.window.add(terminal)
        self.childpid = self.terminal.fork_command(
            self.command[0], self.command, os.getcwd())
        if self.childpid > 0:
            self.childexited = self.terminal.connect(
                'child-exited', self.on_child_exited)
            self.window.connect('delete-event', lambda w, e: gtk.main_quit())
        else:
            self.childexited = None

        spawn_later(0.5, self.show_startup_notify)

        self.window.show_all()

        logo_filename = os.path.join(
            os.path.abspath(os.path.dirname(__file__)), 'goproxy-gtk.png')
        self.window.set_icon_from_file(logo_filename)

        if appindicator:
            self.trayicon = appindicator.Indicator(
                'GoProxy', 'indicator-messages', appindicator.CATEGORY_APPLICATION_STATUS)
            self.trayicon.set_status(appindicator.STATUS_ACTIVE)
            self.trayicon.set_attention_icon('indicator-messages-new')
            self.trayicon.set_icon(logo_filename)
            self.trayicon.set_menu(self.make_menu())
        else:
            self.trayicon = gtk.StatusIcon()
            self.trayicon.set_from_file(logo_filename)
            self.trayicon.connect('popup-menu', lambda i, b, t: self.make_menu().popup(
                None, None, gtk.status_icon_position_menu, b, t, self.trayicon))
            self.trayicon.connect('activate', self.show_hide_toggle)
            self.trayicon.set_tooltip('GoProxy')
            self.trayicon.set_visible(True)

    def make_menu(self):
        menu = gtk.Menu()
        itemlist = [(u'Show', self.on_show),
                    (u'Hide', self.on_hide),
                    (u'Stop', self.on_stop),
                    (u'Reload', self.on_reload),
                    (u'Quit', self.on_quit)]
        for text, callback in itemlist:
            item = gtk.MenuItem(text)
            item.connect('activate', callback)
            item.show()
            menu.append(item)
        menu.show()
        return menu

    def show_notify(self, message=None, timeout=None):
        if pynotify and message:
            notification = pynotify.Notification('GoProxy Notify', message)
            notification.set_hint('x', 200)
            notification.set_hint('y', 400)
            if timeout:
                notification.set_timeout(timeout)
            notification.show()

    def show_startup_notify(self):
        if self.check_child_exists():
            self.show_notify(self.message, timeout=3)

    def check_child_exists(self):
        if self.childpid <= 0:
            return False
        cmd = 'ps -p %s' % self.childpid
        lines = os.popen(cmd).read().strip().splitlines()
        if len(lines) < 2:
            return False
        return True

    def on_child_exited(self, term):
        if self.terminal.get_child_exit_status() == 0:
            gtk.main_quit()
        else:
            self.show_notify(self.fail_message)

    def on_show(self, widget, data=None):
        self.window.show_all()
        self.window.present()
        self.terminal.feed('\r')

    def on_hide(self, widget, data=None):
        self.window.hide_all()

    def on_stop(self, widget, data=None):
        if self.childexited:
            self.terminal.disconnect(self.childexited)
        os.system('kill -9 %s' % self.childpid)

    def on_reload(self, widget, data=None):
        if self.childexited:
            self.terminal.disconnect(self.childexited)
        os.system('kill -9 %s' % self.childpid)
        self.on_show(widget, data)
        self.childpid = self.terminal.fork_command(
            self.command[0], self.command, os.getcwd())
        self.childexited = self.terminal.connect(
            'child-exited', lambda term: gtk.main_quit())

    def show_hide_toggle(self, widget, data=None):
        if self.window.get_property('visible'):
            self.on_hide(widget, data)
        else:
            self.on_show(widget, data)

    def delete_event(self, widget, data=None):
        self.on_hide(widget, data)
        return True

    def on_quit(self, widget, data=None):
        gtk.main_quit()


def main():
    global __file__
    __file__ = os.path.abspath(__file__)
    if os.path.islink(__file__):
        __file__ = getattr(os, 'readlink', lambda x: x)(__file__)
    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    if os.path.isfile('goproxy-gtk.desktop'):
        rewrite_desktop('goproxy-gtk.desktop')

    window = gtk.Window()
    terminal = vte.Terminal()
    GoProxyGTK(window, terminal)
    gtk.main()

if __name__ == '__main__':
    main()
