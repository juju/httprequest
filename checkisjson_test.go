package httprequest_test

import (
	"net/http"
	"strings"

	gc "gopkg.in/check.v1"

	"github.com/juju/httprequest"
)

type checkIsJSONSuite struct{}

var _ = gc.Suite(&checkIsJSONSuite{})

var checkIsJSONTests = []struct {
	about       string
	contentType string
	body        string
	expectError string
}{{
	about:       "simple json",
	contentType: "application/json",
	body:        "not json but unread",
}, {
	about:       "simple json with charset",
	contentType: "application/json; charset=UTF-8",
	body:        "not json but unread",
}, {
	about:       "plain text",
	contentType: "text/plain; charset=UTF-8",
	body:        "   some\n   text\t\n",
	expectError: `unexpected content type text/plain; want application/json; content: some; text`,
}, {
	about:       "plain text with leading newline",
	contentType: "text/plain; charset=UTF-8",
	body:        "\nsome text",
	expectError: `unexpected content type text/plain; want application/json; content: some text`,
}, {
	about:       "unknown content type",
	contentType: "something",
	body:        "some  \nstuff",
	expectError: `unexpected content type something; want application/json; content: "some  \\nstuff"`,
}, {
	about:       "bad content type",
	contentType: "/; charset=foo",
	body:        `some stuff`,
	expectError: `unexpected content type "/; charset=foo"; want application/json; content: "some stuff"`,
}, {
	about:       "large text body",
	contentType: "text/plain",
	body:        strings.Repeat("x", 1024+300),
	expectError: `unexpected content type text/plain; want application/json; content: ` + strings.Repeat("x", 1024) + ` \.\.\. \[300 bytes omitted]`,
}, {
	about:       "html with no text",
	contentType: "text/html",
	body:        "<html><a href=\"foo\">\n</a></html>",
	expectError: `unexpected content type text/html; want application/json; content: "<html><a href=\\"foo\\">\\n</a></html>"`,
}, {
	about:       "non-utf8 text",
	contentType: "text/plain; charset=iso8859-1",
	body:        "Pepp\xe9\n",
	// It would be nice to make this better, but we don't
	// really want to drag in all the charsets for this.
	expectError: "unexpected content type text/plain; want application/json; content: Pepp\uFFFD",
}, {
	about:       "actual html error message from proxy",
	contentType: "text/html; charset=UTF-8",
	body: `<!DOCTYPE HTML PUBLIC "-//IETF//DTD HTML 2.0//EN">
<html><head>
<title>502 Proxy Error</title>
</head><body>
<h1>Proxy Error</h1>
<p>The proxy server received an invalid
response from an upstream server.<br />
The proxy server could not handle the request <em><a href="/identity/v1/wait">GET&nbsp;/identity/v1/wait</a></em>.<p>
Reason: <strong>Error reading from remote server</strong></p></p>
<hr>
<address>Apache/2.4.7 (Ubuntu) Server at api.jujucharms.com Port 443</address>
</body></html>`,
	expectError: `unexpected content type text/html; want application/json; content: 502 Proxy Error; Proxy Error; The proxy server received an invalid response from an upstream server; The proxy server could not handle the request; GET /identity/v1/wait; Reason:; Error reading from remote server; Apache/2\.4\.7 \(Ubuntu\) Server at api.jujucharms.com Port 443`,
}, {
	about:       "actual html error message web page",
	contentType: "text/html; charset=UTF-8",
	body: `<!doctype html>
<!--[if lt IE 7]> <html class="lt-ie9 lt-ie8 lt-ie7" lang="en" dir="ltr"> <![endif]-->
<!--[if IE 7]>    <html class="lt-ie9 lt-ie8" lang="en" dir="ltr"> <![endif]-->
<!--[if IE 8]>    <html class="lt-ie9" lang="en" dir="ltr"> <![endif]-->
<!--[if gt IE 8]><!--> <html class="no-js" lang="en" dir="ltr" itemscope itemtype="http://schema.org/Product"> <!--<![endif]-->
<head>

<title>Page not found | Juju</title>

<meta charset="UTF-8" />
<meta name="description" content="Deploy, manage and scale your environments on any cloud." />
<meta name="keywords" content="juju, charms, services, service orchestration, cloud, deployment, puppet, chef, fuel, cloud tools, service management, cloud management, configuration management, linux tool, openstack tool, ubuntu, cloud design, cloud orchestration" />


<meta name="author" content="Canonical" />
<meta name="viewport" content="width=device-width, initial-scale=1" />

<!--[if IE]>
<meta http-equiv="X-UA-Compatible" content="IE=8">
<![endif]-->

<link rel="shortcut icon" href="/static/img/favicon.ico" type="image/x-icon" />

<link rel="apple-touch-icon-precomposed" sizes="57x57" href="/static/img/icons/apple-touch-icon-57x57-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="60x60" href="/static/img/icons/apple-touch-icon-60x60-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="72x72" href="/static/img/icons/apple-touch-icon-72x72-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="76x76" href="/static/img/icons/apple-touch-icon-76x76-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="114x114" href="/static/img/icons/apple-touch-icon-114x114-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="120x120" href="/static/img/icons/apple-touch-icon-120x120-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="144x144" href="/static/img/icons/apple-touch-icon-144x144-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="152x152" href="/static/img/icons/apple-touch-icon-152x152-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" sizes="180x180" href="/static/img/icons/apple-touch-icon-180x180-precomposed.png"/>
<link rel="apple-touch-icon-precomposed" href="/static/img/icons/apple-touch-icon-precomposed.png"/>


<!-- google fonts -->
<link href='https://fonts.googleapis.com/css?family=Ubuntu:400,300,300italic,400italic,700,700italic%7CUbuntu+Mono' rel='stylesheet' type='text/css' />

<!-- stylesheets -->
<link rel="stylesheet" type="text/css" media="screen" href="//assets.ubuntu.com/sites/guidelines/css/responsive/latest/ubuntu-styles.css" />
<link rel="stylesheet" type="text/css" media="screen" href="/static/css/build.css" />

<!-- load basic yui, our modules file, the loader and sub to set up modules with
combo load -->
<script type="text/javascript"
    
    src="/prod/combo?yui/yui/yui-min.js&amp;app/modules-min.js&amp;yui/loader/loader-min.js&amp;yui/substitute/substitute-min.js&amp;plugins/respond.min.js&amp;plugins/modernizr.2.7.1.js&amp;plugins/handlebars.runtime-v2.0.0.js&amp;plugins/highlight.pack.js&amp;templates/templates.js">
    
</script>

<script type="text/javascript">
      YUI.GlobalConfig = {
          combine: true,
          base: '/prod/combo?yui/',
          comboBase: '/prod/combo?',
          maxURLLength: 1300,
          root: 'yui/',
            groups: {
              app: {
                  combine: true,
                  base: '/prod/combo?app',
                  comboBase: '/prod/combo?',
                  root: 'app/',
                  
                  filter: 'min',
                  
                  // From modules.js
                  modules: YUI_MODULES,
              }
          },
          static_root:'/static/'
    };
 </script>

<!-- provide charmstore url -->
<script type="text/javascript">
    window.csUrl = "https://api.jujucharms.com/charmstore/v4";
</script>


<meta name="twitter:card" content="summary">
<meta name="twitter:site" content="@ubuntucloud">
<meta name="twitter:creator" content="@ubuntucloud">
<meta name="twitter:domain" content="jujucharms.com">
<meta name="twitter:title" content="Search results for foo in Juju">
<meta name="twitter:description" content="Deploy, manage, and scale your environments on any cloud.">
<meta name="twitter:image" content="https://jujucharms.com/static/img/juju-twitter.png">


</head>

<body class="
no-results no-match">

<header class="banner global" role="banner">

    <nav role="navigation" class="nav-primary nav-right">
    

    <span class="accessibility-aid">
        <a accesskey="s" href="#main-content">Jump to content</a>
    </span>
    <ul>
        <li>
    <a
        class=""
        href="/store"
    >
       Store
    </a>
    </li>
        <li><a href="https://demo.jujucharms.com" target="_blank" >Demo</a></li>
        <li>
    <a
        class=""
        href="/about"
    >
       About
    </a>
    </li>
        <li>
    <a
        class=""
        href="/about/features"
    >
       Features
    </a>
    </li>
        <li>
    <a
        class=""
        href="/community"
    >
       Community
    </a>
    </li>
        <li>
    <a
        class=""
        href="/docs/"
    >
       Docs
    </a>
    </li>
        <li>
    <a
        class=""
        href="/get-started"
    >
       Get started
    </a>
    </li>
    </ul>
    <ul class="user-nav">
        <li class="user-dropdown"><span id="user-dropdown"></span></li>
    </ul>
    <a href="#canonlist" class="nav-toggle no-script">☰</a>
</nav>

</header>



<div class="contextual-bar">
    
    <div class="logo">
        <a class="logo-ubuntu" href="/">
            <img width="73" height="30" src="/static/img/logos/logo.png" alt="" />
        </a>
    </div>
    <ul class="directory-path">
        <li><a href="https://demo.jujucharms.com" class="btn__see--more">Create <span>+</span></a></li>
    </ul>

    <form class="search-form" action="/q" method="GET">
    <input
        type="search" name="text"
        class="form-text" placeholder="Search the store"
        value="foo"
    />
    <button type="submit">
        <img
            src="/static/img/icons/search_16_active.svg"
            alt="Search" height="28"
        />
    </button>
    <a href="" class="search-close"><img src="/static/img/icons/close_16.svg" alt="" /></a>
</form>


</div>

<div class="wrapper">
<div id="main-content">





    
    <div class="row">
        <div class="inner-wrapper">
            <h2 class="error-title">404: Sorry, we couldn&rsquo;t find the page.</h2>
            
            <p class="cta">Try a different URL, try searching for solutions or learn how to <a href="/docs/authors-charm-writing">create your own solution</a>.</p>
            <nav class="error-nav">
                <ul>
                    <li><a href="/store" class="link-cta-positive">Browse the store</a></li>
                    <li><a href="/q/?type=bundle" class="link-cta-negative">All bundles</a></li>
                    <li><a href="/q/?type=charm" class="link-cta-negative">All charms</a></li>
                    <li><a href="https://github.com/CanonicalLtd/jujucharms.com/issues">Submit a bug</a></li>
                </ul>
            </nav>
            
        </div>
    </div>



</div><!-- /.inner-wrapper -->

</div><!-- /.wrapper -->


<div class="footer-wrapper strip-light">
<div class="store-cta">
    <a href="/store">Browse the store&nbsp;&rsaquo;</a>
</div>
<footer class="global clearfix">
    <p class="top-link">
        <a href="#">Back to the top</a>
    </p>
    <nav role="navigation" class="inner-wrapper">
        <div class="row">
            <div class="seven-col">
                <ul class="no-bullets">
                    <li><a class="external" href="https://demo.jujucharms.com">Demo</a></li>
                    <li><a href="/about">About</a></li>
                    <li><a href="/about/features">Features</a></li>
                    <li><a href="/docs">Docs</a></li>
                    <li><a href="/get-started">Get Started</a></li>
                </ul>
            </div>
            <div class="five-col last-col">
                <ul class="social--right">
                    <li class="social__item">
                        <a href="https://plus.google.com/106305712267007929608/posts" class="social__google"><span class="accessibility-aid">Juju on Google+</span></a>
                    </li>
                    <li class="social__item">
                        <a href="http://www.twitter.com/ubuntucloud" class="social__twitter"><span class="accessibility-aid">Ubuntu Cloud on Twitter</span></a>
                    </li>
                    <li class="social__item">
                        <a href="http://www.facebook.com/ubuntucloud" class="social__facebook"><span class="accessibility-aid">Ubuntu Cloud on Facebook</span></a>
                    </li>
                </ul>
            </div>
        </div>
    </nav>
    <div class="legal clearfix">
        <div class="legal-inner">
            <p class="twelve-col">
                &copy; 2015 Canonical Ltd. Ubuntu and Canonical are registered trademarks of Canonical Ltd.
            </p>
            <ul class="inline bullets clear">
                <li><a href="http://www.ubuntu.com/legal">Legal information</a></li>
                <li><a href="https://github.com/CanonicalLtd/jujucharms.com/issues">Report a bug on this site</a></li>
            </ul>
            <span class="accessibility-aid">
                <a href="#">Got to the top of the page</a>
            </span>
        </div>
    </div>
</footer>
</div>

<script>
    var isOperaMini = (navigator.userAgent.indexOf('Opera Mini') > -1);
    if(isOperaMini) {
        var root = document.documentElement;
        root.className += " opera-mini";
    }
</script>

<script>
YUI().use('storefront-cookie', 'storefront-utils', 'user-dropdown',
          function (Y) {
    Y.on('domready', function() {
        var inSession = false;
        var cookie = new Y.storefront.CookiePolicy();
        var utils = Y.storefront.utils;

        cookie.render();
        utils.mobileNav(Y.one('header.banner'));
        utils.setupSearch();
        var userDropdown = new Y.storefront.UserDropdownView({
            container: Y.one('#user-dropdown')
        });
        userDropdown.set('authenticated', inSession);
        
        if (inSession) {
            userDropdown.set('username', '');
        }
        userDropdown.render();
    });
});
</script>


<script type="text/template" id="cookie-warning-template">
    <div class="cookie-policy">
        <div class="inner-wrapper">
            <a href="?cp=close" class="link-cta">Close</a>
            <p>We use cookies to improve your experience. By your continued use of this site you accept such use. To change your settings please <a href="http://www.ubuntu.com/legal/terms-and-policies/privacy-policy#cookies">see our policy</a>.</p>
        </div>
    </div>
</script>


<script>
    YUI().use('node', 'search-view', function (Y) {
        Y.on('domready', function() {
            var view = new Y.storefront.SearchView({
                container: Y.one('#main-content')
            });
            view.render();
        });
    });
</script>


<!-- google analytics -->
<script>
  var _gaq = _gaq || [];
  _gaq.push(['_setAccount', 'UA-1018242-44']);
  _gaq.push(['_trackPageview']);

  (function() {
  var ga = document.createElement('script'); ga.type = 'text/javascript'; ga.async = true;
  ga.src = ('https:' == document.location.protocol ? 'https://' : 'http://') + 'stats.g.doubleclick.net/dc.js';
  var s = document.getElementsByTagName('script')[0]; s.parentNode.insertBefore(ga, s);
  })();
</script>


<!-- {version: ['0.1.29', '']} -->

</body>
</html>
`,
	expectError: `unexpected content type text/html; want application/json; content: Page not found | Juju; Jump to content; Store; Demo; About; Features; Community; Docs; Get started; ☰; Create; \+; 404: Sorry, we couldn’t find the page; Try a different URL, try searching for solutions or learn how to; create your own solution; Browse the store; All bundles; All charms; Submit a bug; Browse the store ›; Back to the top; Demo; About; Features; Docs; Get Started; Juju on Google+; Ubuntu Cloud on Twitter; Ubuntu Cloud on Facebook; © 2015 Canonical Ltd. Ubuntu and Canonical are registered trademarks of Canonical Ltd; Legal information; Report a bug on this site; Got to the top of the page`,
}}

func (checkIsJSONSuite) TestCheckIsJSON(c *gc.C) {
	*httprequest.MaxErrorBodySize = 16 * 1024
	for i, test := range checkIsJSONTests {
		c.Logf("test %d: %s", i, test.about)
		r := strings.NewReader(test.body)
		err := httprequest.CheckIsJSON(http.Header{
			"Content-Type": {test.contentType},
		}, r)
		if test.expectError == "" {
			c.Assert(err, gc.IsNil)
			c.Assert(r.Len(), gc.Equals, len(test.body))
			continue
		}
		c.Assert(err, gc.ErrorMatches, test.expectError)
		if len(test.body) > *httprequest.MaxErrorBodySize {
			c.Assert(r.Len(), gc.Equals, *httprequest.MaxErrorBodySize-len(test.body))
		}
	}
}
