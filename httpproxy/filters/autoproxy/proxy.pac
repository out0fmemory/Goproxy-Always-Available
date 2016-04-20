function FindProxyForURL(url, host) {
    if (shExpMatch(host, '*.google*.*') ||
       dnsDomainIs(host, '.ggpht.com') ||
       dnsDomainIs(host, '.wikipedia.org') ||
       host == 'cdnjs.cloudflare.com' ||
       host == 'wp.me' ||
       host == 'po.st' ||
       host == 'goo.gl') {
        return 'PROXY 127.0.0.1:8087';
    }
    return 'DIRECT';
}
