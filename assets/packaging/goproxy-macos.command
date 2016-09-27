(exec /usr/bin/env python2.7 -x "$0" 2>&1 >/dev/null &);exit
# coding:utf-8
# Contributor:
#      Phus Lu        <phuslu@hotmail.com>

__version__ = '1.6'

GOPROXY_TITLE = "GoProxy macOS"
GOPROXY_ICON_DATA = """\
iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAD3ElEQVRoQ+2ZWchN
URTHf58hY2YZQlHGB1NIhlJmEsULyoMpKR4oSciYUDLkRZQylQcZMhOZx1A8kAxJ
SiRjyNS/9qfvu+45e+17Nl+3rLpPd+21/v+z9lnTKaHIpaTI8fOfQEVHMHYEGgB9
gY5Aa6AuUAP4ArwGngB3gCvAuxjkYxBoBEx0vx5gupbfgXPALmAP8KlQMlkINAEW
ANPcUy4UgyKzFtgAfA41UggBnZkOrHZXJNRnkv5DYDJwPsRgKAHd6e3AmBAnAbo/
XFTXAD8t50IINAOOAZ0thjPqDHe+vGasBBoDF4B2XovZFRYDy61mLASqu4zR02o0
g14QePmxENgMzDSCUno8DOwHrgFPXYqsDbRxNWIcMCCPvWDwFgIDgVNG8MrpC12x
8h3pDmwC+jjFgsD7CFQB7gLtPWjeA5PcU/cBL/t/ZWAl8DHkzuc6SLtCqq47PYje
AoOAGyHIY+qmEbgFdE1xppwt8GdiAgq1lUSgC3DbY2yVKzqhPqPqJxFYBixK8fTS
dZsFN2GxWCQRUNFSW5wkS4ClsUBksZOPgLLDB0AFLEnaAmq+LDIbGGJR9Oh8Bcbm
9kj5CGgQeZRi7BnQKgDQVmBKgH6aanPgRVmFfARUXC6mWFFDp2bLKjEJaGC66SOg
cB9PQbfDFa6KIKC0fbqYCQwFTvgI+K6QojPM+viBmFeov2vrf7sv5CV+DrS0TkyR
Cagz0FYjlYAljXYA7hujsMXNuhZ1+U6TeoD6r1QC+lODdb8USys8ldoCNldH+yOB
q5pw+BWgybCcJFViVVn16EmiVYjqhVrpWDLa05IfAUZaCWhwL3fX8qBcB8yNhR44
6brbJJPz3SrHFAEp+dpprT1GWLcHHqKjgIMeHQ1WD6wRkN4Et/pLs6ueSYXvcoZI
CNglQHvVJLkO9Mr3Z9pAo5FS16iTB5xa6qluxxnKQ6AOAE09BzUd7g4lIH1tD6wT
1z434FjSa0NgHjAH0INKk3tuMvxWCAGd2QjMMj5avRd6GbVWueo2FIpQLaAF0A1Q
O6CMo7RpkcFpmxHLXqgacBbobfEWWUerF80TiWIhoMP6BqDipgr8r0QPTT2XPo5k
JiADetGOejYVschpqyfwb3wGrREotVMH2AZoPfi35JBL4UrRXgklIIM6oxFRO/z6
Xg92Bb3sWk2uD+h0TcvdJAhqrFTeZwA17Tj/0FR61AZQvZfm7SApJAK5DpTTx7uP
fCpMlYwIVCT3AhpRg4GX+ohBoCxe9eua6PSZVev00s+suh76rPrYzRFqPdQeZ5bY
BDIDCjXwn0DoE4utX/QR+AWJ6qQxh9YO5QAAAABJRU5ErkJggg=="""

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


class GoProxyMacOS(NSObject):

    console_color=ColorSet[0]

    def applicationDidFinishLaunching_(self, notification):
        self.setupUI()
        self.startGoProxy()
        self.notify()
        self.registerObserver()

    def windowWillClose_(self, notification):
        self.stopGoProxy()
        NSApp.terminate_(self)

    def setupUI(self):
        self.statusbar = NSStatusBar.systemStatusBar()
        # Create the statusbar item
        self.statusitem = self.statusbar.statusItemWithLength_(NSVariableStatusItemLength)
        # Set initial image
        raw_data = base64.b64decode(''.join(GOPROXY_ICON_DATA.strip().splitlines()))
        self.image_data = NSData.dataWithBytes_length_(raw_data, len(raw_data))
        self.image = NSImage.alloc().initWithData_(self.image_data)
        self.image.setSize_((18, 18))
        self.image.setTemplate_(True)
        self.statusitem.setImage_(self.image)
        # Let it highlight upon clicking
        self.statusitem.setHighlightMode_(1)
        # Set a tooltip
        self.statusitem.setToolTip_(GOPROXY_TITLE)

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
        self.console_window.setTitle_(GOPROXY_TITLE)
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

    def startGoProxy(self):
        cmd = '%s/goproxy' % os.path.dirname(os.path.abspath(__file__))
        self.master, self.slave = pty.openpty()
        self.pipe = subprocess.Popen(cmd, shell=True, stdin=subprocess.PIPE, stdout=self.slave, stderr=self.slave, close_fds=True)
        self.pipe_fd = os.fdopen(self.master)
        self.performSelectorInBackground_withObject_('readProxyOutput', None)

    def notify(self):
        from Foundation import NSUserNotification, NSUserNotificationCenter
        notification = NSUserNotification.alloc().init()
        notification.setTitle_("GoProxy macOS Started.")
        notification.setSubtitle_("")
        notification.setInformativeText_("")
        notification.setSoundName_("NSUserNotificationDefaultSoundName")
        notification.setContentImage_(self.image)
        NSUserNotificationCenter.defaultUserNotificationCenter().scheduleNotification_(notification)

    def stopGoProxy(self):
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
        self.show_(True)
        self.stopGoProxy()
        self.console_view.setString_('')
        self.startGoProxy()

    def exit_(self, notification):
        self.stopGoProxy()
        NSApp.terminate_(self)


def main():
    global __file__
    __file__ = os.path.abspath(__file__)
    if os.path.islink(__file__):
        __file__ = getattr(os, 'readlink', lambda x: x)(__file__)
    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    app = NSApplication.sharedApplication()
    delegate = GoProxyMacOS.alloc().init()
    app.setDelegate_(delegate)

    AppHelper.runEventLoop()

if __name__ == '__main__':
    main()
