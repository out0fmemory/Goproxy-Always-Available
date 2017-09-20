#!/usr/bin/env python2
# coding:utf-8

import sys
import os

sys.dont_write_bytecode = True

if sys.version > '3.':
    sys.exit(sys.stderr.write('Please run uploader.py by python2\n'))

os.chdir(os.path.abspath(os.path.dirname(__file__)))
CACHE_DIR = 'cache'

import re
import socket
import traceback
import shutil
import functools
import ssl
import mimetypes
import multiprocessing.pool

mimetypes._winreg = None

def clear():
    if os.name == 'nt':
        os.system('cls')
    else:
        sys.stderr.write("\x1b[2J\x1b[H")

def println(s, file=sys.stderr):
    assert type(s) is type(u'')
    file.write(s.encode(sys.getfilesystemencoding(), 'replace') + os.linesep)

try:
    socket.create_connection(('127.0.0.1', 8087), timeout=0.5).close()
    os.environ['HTTP_PROXY'] = 'http://127.0.0.1:8087'
    os.environ['HTTPS_PROXY'] = 'http://127.0.0.1:8087'
    println(u'使用 HTTP 代理：127.0.0.1:8087')
except socket.error:
    try:
        socket.create_connection(('127.0.0.1', 1080), timeout=0.5).close()
        sys.path.append('PySocks')
        import socks
        if os.name == 'nt':
            import win_inet_pton
        socks.set_default_proxy(socks.SOCKS5, '127.0.0.1', port=1080)
        socket.socket = socks.socksocket
        println(u'使用 SOCKS5 代理：127.0.0.1:1080')
    except socket.error:
        println(u'''\
警告：检测到本机没有在指定端口监听的 HTTP 代理 (8087) 或 SOCKS5 代理 (1080)，
      建议先启动 GoProxy 客户端或者其它代理，并根据代理类型设定监听的端口。

如果你使用的是 VPN 并且已经正常工作的话，请忽略此警告，按回车键继续。''')
        raw_input()


def patch_google_appengine_sdk(root_dir, *patch_list):
    for item in patch_list:
        filename = os.path.normpath(os.path.join(root_dir, item['name']))
        try:
            with open(filename, 'rb') as fp:
                text = fp.read()
            if item['old'] in text:
                println(u'patch_google_appengine_sdk(%s)' % filename)
                with open(filename, 'wb') as fp:
                    fp.write(text.replace(item['old'], item['new']))
        except Exception as e:
            println(u'patch_google_appengine_sdk(%s) error: %s' % (filename, e))

patch_google_appengine_sdk('./google_appengine',
    {
        'name': 'google/appengine/tools/appengine_rpc_httplib2.py',
        'old': '~/.appcfg_oauth2_tokens',
        'new': './.appcfg_oauth2_tokens',
    },
    {
        'name': 'httplib2/__init__.py',
        'old': 'self.proxy_rdns = proxy_rdns',
        'new': 'self.proxy_rdns = True',
    },
    {
        'name': 'httplib2/__init__.py',
        'old': 'content = zlib.decompress(content)',
        'new': 'content = zlib.decompress(content, -zlib.MAX_WBITS)',
    })


sys.path = ['google_appengine'] + sys.path

import httplib2
def _ssl_wrap_socket(sock, key_file, cert_file,
                     disable_validation, ca_certs):
    cert_reqs = ssl.CERT_NONE
    return ssl.wrap_socket(sock, keyfile=key_file, certfile=cert_file,
                           cert_reqs=ssl.CERT_NONE, ca_certs=None,
                           ssl_version=ssl.PROTOCOL_TLSv1)
httplib2._ssl_wrap_socket = _ssl_wrap_socket
httplib2.HTTPSConnectionWithTimeout._ValidateCertificateHostname = lambda a, b, c: True
if hasattr(ssl, '_create_unverified_context'):
    setattr(ssl, '_create_default_https_context', ssl._create_unverified_context)

println(u'Loading Google Appengine SDK...')
from google_appengine.google.appengine.tools import appcfg

def upload(dirname, appid):
    assert isinstance(dirname, basestring) and isinstance(appid, basestring)
    oldname = dirname
    dirname = '%s/%s-%s' % (CACHE_DIR, dirname, appid)
    if os.path.isdir(dirname):
        shutil.rmtree(dirname, ignore_errors=True)
    shutil.copytree(oldname, dirname)
    filename = os.path.join(dirname, 'app.yaml')
    with open(filename, 'rb') as fp:
        content = fp.read()
    with open(filename, 'wb') as fp:
        fp.write(re.sub(r'application:.*', 'application: '+appid, content))
    if os.name == 'nt':
        appcfg.main(['appcfg', 'rollback', dirname])
        appcfg.main(['appcfg', 'update', dirname])
    else:
        appcfg.main(['appcfg', 'rollback', '--noauth_local_webserver', dirname])
        appcfg.main(['appcfg', 'update', '--noauth_local_webserver', dirname])

def retry_upload(max_retries, dirname, appid):
    for _ in xrange(max_retries):
        try:
            upload(dirname, appid)
            break
        except (Exception, SystemExit) as e:
            println(u'=====上传 APPID(%r) 失败，重试中...=====' % appid)

def input_appids():
    while True:
        appids = [x.strip().lower() for x in raw_input('APPID:').split('|')]
        ok = True
        for appid in appids:
            if not re.match(r'^[0-9a-zA-Z\-|]+$', appid):
                println(u'appid(%s) 格式错误，请登录 https://console.cloud.google.com/appengine 查看您的 appid!' % appid)
                ok = False
            if any(x in appid for x in ('ios', 'android', 'mobile')):
                println(u'appid(%s) 不能包含 ios/android/mobile 等字样。' % appid)
                ok = False
        if ok:
            return appids

def main():
    clear()
    println(u'''\
===============================================================
 GoProxy 服务端部署程序, 开始上传 gae 应用文件夹
 Linux/Mac 用户, 请使用 python uploader.py 来上传应用
===============================================================

请输入您的appid, 多个appid请用|号隔开
特别提醒：appid 请勿包含 ID/Email 等个人信息！
        '''.strip())
    if not os.path.isdir(CACHE_DIR):
        os.mkdir(CACHE_DIR)
    appids = input_appids()
    retry_upload(4, 'gae', appids[0])
    pool = multiprocessing.pool.ThreadPool(processes=50)
    pool.map(functools.partial(retry_upload, 4, 'gae'), appids[1:])
    shutil.rmtree(CACHE_DIR, ignore_errors=True)
    println(os.linesep + u'上传完毕，请检查 http://<appid>.appspot.com 的版本，谢谢。按回车键退出程序。')
    raw_input()


if __name__ == '__main__':
    try:
        main()
    except:
        traceback.print_exc()
        raw_input()
