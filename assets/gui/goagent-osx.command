(exec /usr/bin/env python2.7 -x "$0" 2>&1 >/dev/null &);exit
# coding:utf-8
# Contributor:
#      Phus Lu        <phuslu@hotmail.com>

__version__ = '1.6'

GOAGENT_TITLE = "GoAgent OS X"
GOAGENT_ICON_DATA = """\
iVBORw0KGgoAAAANSUhEUgAAABQAAAAUCAMAAAC6V+0/AAAABGdBTUEAALGPC/xhBQAAAAFzUkdC
AK7OHOkAAAAgY0hSTQAAeiYAAICEAAD6AAAAgOgAAHUwAADqYAAAOpgAABdwnLpRPAAAArtQTFRF
AAAAeHh4e3t7aWlpODg4gICAYmJifX19UVFRd3d3b29vT09PExMTdnZ2dXV10tLScHBwFRUVx8fH
0dHRzc3NfHx80NDQcnJyysrKzs7OzMzMdHR08PDwMzMzjY2N1NTUo6Oj////09PT1tbWtra229vb
2travr6+5+fn3t7eKCgocHBwbGxsa2trbGxsa2traGhoaGhoZ2dnampqbGxsbGxsaWlpaGhoZmZm
ZWVlZWVlZmZmYGBgbm5ubW1tbGxsaWlpZWVlZ2dnbGxsbW1ta2traWlpZ2dnZWVlZWVlZ2dnc3Nz
cXFxbm5ubW1tbm5ua2trampqaWlpZmZmZWVljIyM09PTz8/Py8vLcXFxb29vbW1tbGxsa2trampq
aGhoZ2dnZWVlY2NjsrKy0NDQzs7Ozs7Ou7u7cnJyb29vY2NjtLS0zc3Nzc3Ny8vLcnJybm5uZWVl
xcXFy8vLzMzMdHR0bm5uaWlpbGxsb29vbm5ubW1tbGxscHBwrq6uz8/PzMzMz8/Pc3Nza2trmpqa
0tLS0tLS0NDQz8/Pz8/P0NDQz8/PcXFxbm5ueXl52dnZy8vLzMzMc3NzampqiYmJ3Nzczc3NzMzM
cHBwb29vbW1tjo6O29vb1NTU1dXV1dXV09PT0tLS0dHR0NDQzs7Ozc3NdnZ2c3NzbW1tvLy82tra
1NTU09PT0tLS0tLS0tLS0tLS0tLS0NDQx8fH3Nzc2tra0tLS0dHR09PT0tLS29vb2dnZ1NTU0dHR
0tLSz8/P0NDQ2dnZ19fX1dXV1dXV09PT0tLS09PT09PTsrKy4+Pj2NjY1tbW2NjY1tbW0tLS0dHR
0tLSZmZmZWVlZGRkY2NjbGxsa2trampqaWlpaGhoy8vLZ2dnzc3NzMzM1tbW1NTU09PT0tLS0dHR
z8/Pzs7O1dXV0NDQ////wMAjVgAAANJ0Uk5TAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAo1V2dnVS4FCoq/5vz7+eNrAyfDPJ3UHSnVs9j5/t4nARouQIaUkZ7u3Tsu
JQVQzOjo5eXl6PvZi+jcZAEczteT/tAbRfS2pPJBYfroua2tra2cjej6Xl/ghqm7u7vG72FB8py9
8UIa/ZPbxhZTy9yG3Pnc19fY3N3OUwEYJDng7aCUloY2JRoCKeHTqNUnHM+aRbYcAVLK8/v75axQ
AQEYQ2JiQxoB7jQAFgAAAAFiS0dEIcRsDRYAAAAJcEhZcwAAAEgAAABIAEbJaz4AAAFTSURBVBjT
Y2DABRiZtHV09fQNDI2Y4WIsrMYmpmbmFpZW1jZs7FBBDk5bO3uHS5evXHV04uKGCvLwOru4url7
XLnq6cUHEeL09vH18w8IDAq+cjUkNCw8gp+BQUAwMio6JjYuPiHxytWk5JTUtHQhBt6MzGvXb9y8
dQlkZlZ2zu3cPGEGvvwCsOAdkGBh0d3bxSUiDKKlZUDB8orKquqa2rr6e7cbGsUYxJuAgs0trW1t
7R2dXXeBgt0SDOI9vdeu9/Xff/Dw0eMnT4GCEyZKMvBNiro2ecrUZw8fPn4OFpw2XYpBmmHGzFmz
5zx7OHfe/AULFy1eslRGhEFWbtnyFStXPXu4es3ades3bNy0WV4B5CdFpS1bgdofbdu+Y6eyCtTv
qmq7doME9+zdt19ZHSqooXng4KHDR44eO37ipIYWPEQ1T50+c/bc+QsXlXBGBAMAuICbVLgeGx4A
AAAldEVYdGRhdGU6Y3JlYXRlADIwMTUtMDctMDZUMTM6NDg6NTgrMDg6MDAVL5yCAAAAJXRFWHRk
YXRlOm1vZGlmeQAyMDE1LTA3LTA2VDEzOjQ2OjQ5KzA4OjAwEGYfpwAAAABJRU5ErkJggg=="""

import sys
import subprocess
import pty
import os
import base64
import ctypes
import ctypes.util

from PyObjCTools import AppHelper
from AppKit import *

ColorSet=[NSColor.blackColor(), NSColor.colorWithDeviceRed_green_blue_alpha_(0.7578125,0.2109375,0.12890625,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.14453125,0.734375,0.140625,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.67578125,0.67578125,0.15234375,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.28515625,0.1796875,0.87890625,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.82421875,0.21875,0.82421875,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.19921875,0.73046875,0.78125,1.0), NSColor.colorWithDeviceRed_green_blue_alpha_(0.79296875,0.796875,0.80078125,1.0)]


class GoAgentOSX(NSObject):

    console_color=ColorSet[0]

    def applicationDidFinishLaunching_(self, notification):
        self.setupUI()
        self.startGoAgent()
        self.notify()
        self.registerObserver()

    def windowWillClose_(self, notification):
        self.stopGoAgent()
        NSApp.terminate_(self)

    def setupUI(self):
        self.statusbar = NSStatusBar.systemStatusBar()
        # Create the statusbar item
        self.statusitem = self.statusbar.statusItemWithLength_(NSVariableStatusItemLength)
        # Set initial image
        raw_data = base64.b64decode(''.join(GOAGENT_ICON_DATA.strip().splitlines()))
        self.image_data = NSData.dataWithBytes_length_(raw_data, len(raw_data))
        self.image = NSImage.alloc().initWithData_(self.image_data)
        self.statusitem.setImage_(self.image)
        # Let it highlight upon clicking
        self.statusitem.setHighlightMode_(1)
        # Set a tooltip
        self.statusitem.setToolTip_(GOAGENT_TITLE)

        # Build a very simple menu
        self.menu = NSMenu.alloc().init()
        # Show Menu Item
        menuitem = NSMenuItem.alloc().initWithTitle_action_keyEquivalent_('Show', 'show:', '')
        self.menu.addItem_(menuitem)
        # Hide Menu Item
        menuitem = NSMenuItem.alloc().initWithTitle_action_keyEquivalent_('Hide', 'hide2:', '')
        self.menu.addItem_(menuitem)
        # Rest Menu Item
        menuitem = NSMenuItem.alloc().initWithTitle_action_keyEquivalent_('Reload', 'reset:', '')
        self.menu.addItem_(menuitem)
        # Default event
        menuitem = NSMenuItem.alloc().initWithTitle_action_keyEquivalent_('Quit', 'exit:', '')
        self.menu.addItem_(menuitem)
        # Bind it to the status item
        self.statusitem.setMenu_(self.menu)

        # Console window
        frame = NSMakeRect(0, 0, 550, 350)
        self.console_window = NSWindow.alloc().initWithContentRect_styleMask_backing_defer_(frame, NSClosableWindowMask | NSTitledWindowMask, NSBackingStoreBuffered, False)
        self.console_window.setTitle_(GOAGENT_TITLE)
        self.console_window.setDelegate_(self)

        # Console view inside a scrollview
        self.scroll_view = NSScrollView.alloc().initWithFrame_(frame)
        self.scroll_view.setBorderType_(NSNoBorder)
        self.scroll_view.setHasVerticalScroller_(True)
        self.scroll_view.setHasHorizontalScroller_(False)
        self.scroll_view.setAutoresizingMask_(NSViewWidthSizable | NSViewHeightSizable)

        self.console_view = NSTextView.alloc().initWithFrame_(frame)
        self.console_view.setRichText_(True)
        self.console_view.setVerticallyResizable_(True)
        self.console_view.setHorizontallyResizable_(True)
        self.console_view.setAutoresizingMask_(NSViewWidthSizable)

        self.scroll_view.setDocumentView_(self.console_view)

        contentView = self.console_window.contentView()
        contentView.addSubview_(self.scroll_view)

        # Hide dock icon
        NSApp.setActivationPolicy_(NSApplicationActivationPolicyProhibited)

    def registerObserver(self):
        nc = NSWorkspace.sharedWorkspace().notificationCenter()
        nc.addObserver_selector_name_object_(self, 'exit:', NSWorkspaceWillPowerOffNotification, None)

    def startGoAgent(self):
        cmd = '%s/goproxy -v=2' % os.path.dirname(os.path.abspath(__file__))
        self.master, self.slave = pty.openpty()
        self.pipe = subprocess.Popen(cmd, shell=True, stdin=subprocess.PIPE, stdout=self.slave, stderr=self.slave, close_fds=True)
        self.pipe_fd = os.fdopen(self.master)
        self.performSelectorInBackground_withObject_('readProxyOutput', None)

    def notify(self):
        from Foundation import NSUserNotification, NSUserNotificationCenter
        notification = NSUserNotification.alloc().init()
        notification.setTitle_("GoAgent OSX Started.")
        notification.setSubtitle_("")
        notification.setInformativeText_("")
        notification.setSoundName_("NSUserNotificationDefaultSoundName")
        notification.setContentImage_(self.image)
        NSUserNotificationCenter.defaultUserNotificationCenter().scheduleNotification_(notification)

    def stopGoAgent(self):
        self.pipe.terminate()

    def parseLine(self, line):
        while line.startswith('\x1b['):
            line = line[2:]
            color_number = int(line.split('m',1)[0])
            print color_number
            if 30 <= color_number < 38:
                self.console_color = ColorSet[color_number-30]
            elif color_number == 0:
                self.console_color = ColorSet[0]
            line = line.split('m',1)[1]
        return line

    def refreshDisplay_(self, line):
        line = self.parseLine(line)
        console_line = NSMutableAttributedString.alloc().initWithString_(line)
        console_line.addAttribute_value_range_(NSForegroundColorAttributeName, self.console_color, NSMakeRange(0,len(line)))
        self.console_view.textStorage().appendAttributedString_(console_line)
        need_scroll = NSMaxY(self.console_view.visibleRect()) >= NSMaxY(self.console_view.bounds())
        if need_scroll:
            range = NSMakeRange(len(self.console_view.textStorage().mutableString()), 0)
            self.console_view.scrollRangeToVisible_(range)

    def readProxyOutput(self):
        while(True):
            line = self.pipe_fd.readline()
            self.performSelectorOnMainThread_withObject_waitUntilDone_('refreshDisplay:', line, None)

    def show_(self, notification):
        self.console_window.center()
        self.console_window.orderFrontRegardless()
        self.console_window.setIsVisible_(True)

    def hide2_(self, notification):
        self.console_window.setIsVisible_(False)
        #self.console_window.orderOut(None)

    def reset_(self, notification):
        self.console_view.setString_('')
        self.stopGoAgent()
        self.startGoAgent()

    def exit_(self, notification):
        self.stopGoAgent()
        NSApp.terminate_(self)


def main():
    global __file__
    __file__ = os.path.abspath(__file__)
    if os.path.islink(__file__):
        __file__ = getattr(os, 'readlink', lambda x: x)(__file__)
    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    app = NSApplication.sharedApplication()
    delegate = GoAgentOSX.alloc().init()
    app.setDelegate_(delegate)

    AppHelper.runEventLoop()

if __name__ == '__main__':
    main()
