(/System/Library/Frameworks/Python.framework/Versions/2.7/Resources/Python.app/Contents/MacOS/Python -x "$0" >/dev/null 2>&1 &); exit
#!/usr/bin/python2.7
# coding:utf-8

__version__ = '2.1'

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
import re
import glob
import base64
import ctypes
import ctypes.util

from PyObjCTools import AppHelper
from AppKit import NSAlert
from AppKit import NSApp
from AppKit import NSAppleScript
from AppKit import NSApplication
from AppKit import NSApplicationActivationPolicyProhibited
from AppKit import NSBackingStoreBuffered
from AppKit import NSBezelBorder
from AppKit import NSClosableWindowMask
from AppKit import NSColor
from AppKit import NSData
from AppKit import NSFont
from AppKit import NSForegroundColorAttributeName
from AppKit import NSImage
from AppKit import NSInformationalAlertStyle
from AppKit import NSMakeRange
from AppKit import NSMakeRect
from AppKit import NSMaxY
from AppKit import NSMenu
from AppKit import NSMenuItem
from AppKit import NSMutableAttributedString
from AppKit import NSNoBorder
from AppKit import NSObject
from AppKit import NSScrollView
from AppKit import NSStatusBar
from AppKit import NSTextView
from AppKit import NSTitledWindowMask
from AppKit import NSUserNotification
from AppKit import NSUserNotificationCenter
from AppKit import NSVariableStatusItemLength
from AppKit import NSViewHeightSizable
from AppKit import NSViewWidthSizable
from AppKit import NSWarningAlertStyle
from AppKit import NSWindow
from AppKit import NSWorkspace
from AppKit import NSWorkspaceWillPowerOffNotification

try:
    import setproctitle
    setproctitle.setproctitle(__file__)
except ImportError:
    pass


ColorSet = [NSColor.whiteColor(),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.7578125,0.2109375,0.12890625,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.14453125,0.734375,0.140625,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.67578125,0.67578125,0.15234375,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.28515625,0.1796875,0.87890625,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.82421875,0.21875,0.82421875,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.19921875,0.73046875,0.78125,1.0),
            NSColor.colorWithDeviceRed_green_blue_alpha_(0.79296875,0.796875,0.80078125,1.0)]

ConsoleFont = NSFont.fontWithName_size_("Monaco", 12.0)

class GoProxyHelpers(object):

    def __init__(self):
        self.__network_location = ''

    @property
    def network_location(self):
        if self.__network_location == '':
            s = os.popen('system_profiler SPNetworkDataType').read()
            addrs = re.findall(r'(?is)\s*([^\n]+):\s+Type:\s+(AirPort|Ethernet).+?Addresses:\s*(\S+).+?(?:\n\n|$)', s)
            self.__network_location = next(n for n,t,a in addrs if re.match('^[0-9a-fA-F\.:]+$', a))
        return self.__network_location

    def get_current_proxy(self):
        s = os.popen('scutil --proxy').read()
        info = dict(re.findall('(?m)^\s+([A-Z]\w+)\s+:\s+(\S+)', s))
        if info.get('HTTPEnable') == '1':
            return '%s:%s' % (info['HTTPProxy'], info['HTTPPort'])
        elif info.get('ProxyAutoConfigEnable') == '1':
            return info['ProxyAutoConfigURLString']
        else:
            return '<None>'

    def set_webproxy(self, host, port):
        assert isinstance(host, basestring) and isinstance(port, int)
        cmds = []
        network = self.network_location
        cmds.append('networksetup -setwebproxy %s %s %d' % (network, host, port))
        cmds.append('networksetup -setwebproxystate %s on' % network)
        cmds.append('networksetup -setsecurewebproxy %s %s %d' % (network, host, port))
        cmds.append('networksetup -setsecurewebproxystate %s on' % network)
        cmds.append('networksetup -setautoproxystate %s off' % network)
        script = '''do shell script "%s" with administrator privileges''' % ' && '.join(cmds)
        result, error = NSAppleScript.alloc().initWithSource_(script).executeAndReturnError_(None)
        return result, error

    def set_autoproxy(self, url):
        cmds = []
        network = self.network_location
        cmds.append('networksetup -setautoproxyurl %s %s' % (network, url))
        cmds.append('networksetup -setautoproxystate %s on' % network)
        cmds.append('networksetup -setwebproxystate %s off' % network)
        cmds.append('networksetup -setsecurewebproxystate %s off' % network)
        script = '''do shell script "%s" with administrator privileges''' % ' && '.join(cmds)
        result, error = NSAppleScript.alloc().initWithSource_(script).executeAndReturnError_(None)
        return result, error

    def unset_proxy(self):
        cmds = []
        network = self.network_location
        cmds.append('networksetup -setwebproxystate %s off' % network)
        cmds.append('networksetup -setsecurewebproxystate %s off' % network)
        cmds.append('networksetup -setautoproxystate %s off' % network)
        script = '''do shell script "%s" with administrator privileges''' % ' && '.join(cmds)
        result, error = NSAppleScript.alloc().initWithSource_(script).executeAndReturnError_(None)
        return result, error

    def check_update(self):
        script = '''tell application "Terminal"\nactivate\ndo script "%s/get-latest-goproxy.sh"\nend tell''' % os.getcwd()
        result, error = NSAppleScript.alloc().initWithSource_(script).executeAndReturnError_(None)
        return result, error

    def import_rootca(self, certfile):
        cmds = []
        if not os.path.isfile(certfile):
            raise SystemError('File %r not exists.' % certfile)
        cmds.append('security delete-certificate -c GoProxy')
        cmds.append('security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s' % certfile)
        script = '''do shell script "%s" with administrator privileges''' % ' ; '.join(cmds)
        result, error = NSAppleScript.alloc().initWithSource_(script).executeAndReturnError_(None)
        return result, error


class GoProxyMacOS(NSObject):

    console_color = ColorSet[0]
    max_line_count = 1000

    def applicationDidFinishLaunching_(self, notification):
        self.helper = GoProxyHelpers()
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
        self.menu.addItemWithTitle_action_keyEquivalent_('Show', self.show_, '').setTarget_(self)
        # Hide Menu Item
        self.menu.addItemWithTitle_action_keyEquivalent_('Hide', self.hide2_, '').setTarget_(self)
        # Proxy Menu Item
        self.submenu = NSMenu.alloc().init()
        self.submenu_titles = [
            ('<None>', self.setproxy0_),
            ('http://127.0.0.1:8087/proxy.pac', self.setproxy1_),
            ('127.0.0.1:8087', self.setproxy2_),
        ]
        for title, callback in self.submenu_titles:
            self.submenu.addItemWithTitle_action_keyEquivalent_(title, callback, '').setTarget_(self)
        menuitem = NSMenuItem.alloc().initWithTitle_action_keyEquivalent_('Set Proxy', None, '')
        menuitem.setTarget_(self)
        menuitem.setSubmenu_(self.submenu)
        self.menu.addItem_(menuitem)
        # Rest Menu Item
        self.menu.addItemWithTitle_action_keyEquivalent_('Import RootCA', self.importca_, '').setTarget_(self)
        self.menu.addItemWithTitle_action_keyEquivalent_('Check Update', self.checkupdate_, '').setTarget_(self)
        self.menu.addItemWithTitle_action_keyEquivalent_('Reload', self.reset_, '').setTarget_(self)
        # Default event
        self.menu.addItemWithTitle_action_keyEquivalent_('Quit', self.exit_, '').setTarget_(self)
        # Bind it to the status item
        self.statusitem.setMenu_(self.menu)

        # Console window
        frame = NSMakeRect(0, 0, 640, 480)
        self.console_window = NSWindow.alloc().initWithContentRect_styleMask_backing_defer_(frame, NSClosableWindowMask | NSTitledWindowMask, NSBackingStoreBuffered, False)
        self.console_window.setTitle_(GOPROXY_TITLE)
        self.console_window.setDelegate_(self)

        # Console view inside a scrollview
        self.scroll_view = NSScrollView.alloc().initWithFrame_(frame)
        self.scroll_view.setBorderType_(NSBezelBorder)
        self.scroll_view.setHasVerticalScroller_(True)
        self.scroll_view.setHasHorizontalScroller_(False)
        self.scroll_view.setAutoresizingMask_(NSViewWidthSizable | NSViewHeightSizable)

        self.console_view = NSTextView.alloc().initWithFrame_(frame)
        self.console_view.setBackgroundColor_(NSColor.blackColor())
        self.console_view.setRichText_(True)
        self.console_view.setVerticallyResizable_(True)
        self.console_view.setHorizontallyResizable_(True)
        self.console_view.setAutoresizingMask_(NSViewWidthSizable)
        self.console_line_count = 0

        self.scroll_view.setDocumentView_(self.console_view)
        self.console_window.contentView().addSubview_(self.scroll_view)

        # Update Proxy Menu
        AppHelper.callLater(1, self.updateproxystate_, None)
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
        notification = NSUserNotification.alloc().init()
        notification.setTitle_("GoProxy macOS Started.")
        notification.setSubtitle_("")
        notification.setInformativeText_("")
        notification.setSoundName_("NSUserNotificationDefaultSoundName")
        notification.setContentImage_(self.image)
        usernotifycenter = NSUserNotificationCenter.defaultUserNotificationCenter()
        usernotifycenter.removeAllDeliveredNotifications()
        usernotifycenter.setDelegate_(self)
        usernotifycenter.scheduleNotification_(notification)

    def userNotificationCenter_didActivateNotification_(self, center, notification):
        NSUserNotificationCenter.defaultUserNotificationCenter().removeAllDeliveredNotifications()

    def stopGoProxy(self):
        self.pipe.terminate()

    def parseLine_(self, line):
        if line.startswith('\x1b]2;') and '\x07' in line:
            global GOPROXY_TITLE
            pos = line.find('\x07')
            GOPROXY_TITLE = line[4:pos]
            self.statusitem.setToolTip_(GOPROXY_TITLE)
            self.console_window.setTitle_(GOPROXY_TITLE)
            return self.parseLine_(line[pos:])
        while line.startswith('\x1b['):
            line = line[2:]
            color_number = int(line.split('m',1)[0])
            if 30 <= color_number < 38:
                self.console_color = ColorSet[color_number-30]
            elif color_number == 0:
                self.console_color = ColorSet[0]
            line = line.split('m',1)[1]
        return line

    def refreshDisplay_(self, line):
        line = self.parseLine_(line)
        console_line = NSMutableAttributedString.alloc().initWithString_(line)
        console_line.addAttribute_value_range_(NSForegroundColorAttributeName, self.console_color, NSMakeRange(0,len(line)))
        self.console_view.textStorage().appendAttributedString_(console_line)
        self.console_view.textStorage().setFont_(ConsoleFont)
        need_scroll = NSMaxY(self.console_view.visibleRect()) >= NSMaxY(self.console_view.bounds())
        if need_scroll:
            range = NSMakeRange(len(self.console_view.textStorage().mutableString()), 0)
            self.console_view.scrollRangeToVisible_(range)
        # self.console_view.textContainer().setWidthTracksTextView_(False)
        # self.console_view.textContainer().setContainerSize_((640, 480))

    def readProxyOutput(self):
        while(True):
            line = self.pipe_fd.readline()
            if self.console_line_count > self.max_line_count:
                self.console_view.setString_('')
                self.console_line_count = 0
            self.performSelectorOnMainThread_withObject_waitUntilDone_('refreshDisplay:', line, None)
            self.console_line_count += 1

    def updateproxystate_(self, notification):
        # Add checkmark to submenu
        proxy_title = self.helper.get_current_proxy()
        for title, _ in self.submenu_titles:
            state = 1 if title == proxy_title else 0
            self.submenu.itemWithTitle_(title).setState_(state)

    def setproxy0_(self, notification):
        self.helper.unset_proxy()
        self.updateproxystate_(notification)

    def setproxy1_(self, notification):
        self.helper.set_autoproxy('http://127.0.0.1:8087/proxy.pac')
        self.updateproxystate_(notification)

    def setproxy2_(self, notification):
        self.helper.set_webproxy('127.0.0.1', 8087)
        self.updateproxystate_(notification)

    def importca_(self, notification):
        certfile = './GoProxy.crt'
        self.helper.import_rootca(certfile)

    def checkupdate_(self, notification):
        self.helper.check_update()

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


def get_executables():
    MAXPATHLEN = 1024
    PROC_PIDPATHINFO_MAXSIZE = MAXPATHLEN * 4
    PROC_ALL_PIDS = 1
    libc = ctypes.CDLL(ctypes.util.find_library('c'))
    number_of_pids = libc.proc_listpids(PROC_ALL_PIDS, 0, None, 0)
    pid_list = (ctypes.c_uint32 * (number_of_pids * 2))()
    libc.proc_listpids(PROC_ALL_PIDS, 0, pid_list, ctypes.sizeof(pid_list))
    results = []
    path_size = PROC_PIDPATHINFO_MAXSIZE
    path_buffer = ctypes.create_string_buffer('\0'*path_size,path_size)
    for pid in pid_list:
        # re-use the buffer
        ctypes.memset(path_buffer, 0, path_size)
        return_code = libc.proc_pidpath(pid, path_buffer, path_size)
        if path_buffer.value:
            results.append((pid, path_buffer.value))
    return results


def precheck():
    has_user_json = glob.glob('*.user.json') != []
    if not has_user_json:
        alert = NSAlert.alloc().init()
        alert.setMessageText_('Please configure your goproxy at first.')
        alert.setInformativeText_('For example, add a new gae.user.json')
        alert.setAlertStyle_(NSWarningAlertStyle)
        alert.addButtonWithTitle_('OK')
        NSApp.activateIgnoringOtherApps_(True)
        pressed = alert.runModal()
        os.system('open "%s"' % os.path.dirname(__file__))
    for pid, path in get_executables():
        if path.endswith('/goproxy'):
            os.kill(pid, 9)


def main():
    global __file__
    __file__ = os.path.abspath(__file__)
    if os.path.islink(__file__):
        __file__ = getattr(os, 'readlink', lambda x: x)(__file__)
    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    precheck()
    app = NSApplication.sharedApplication()
    delegate = GoProxyMacOS.alloc().init()
    app.setDelegate_(delegate)

    AppHelper.runEventLoop()

if __name__ == '__main__':
    main()

