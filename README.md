go-idpot
========

[![Build Status](https://travis-ci.org/lestrrat/go-idpot.png?branch=master)](https://travis-ci.org/lestrrat/go-idpot)

Layman's idpot (Serial ID Generator) Using Mysql

Description
===========

This is a dead simple implementation of an "idpot", a server that generates
serial 64-bit unsigned integer IDs. It uses MySQL's MyISAM db and 
LAST\_INSERT\_ID functionality as the basis to generate an ID, along with a 
dead simple HTTP API.

Why/When Would you use this? It's easy to create a 64 bit UID generator without
using MySQL, but it's also really hard to make sure that the ID generation
is fast + correct, AND also make it not go ga-ga. MySQL is, even if you don't
like it as a DB, battlefield-tested server with just enough speed and 
stability. Sometimes it's easier just to slap an easy to use API over what's
already available :)

Once you start the server, you can create a "pot" by POST-ing to 

```
  http://youserver/pot/create
```

With parameters like:

```
  name: NameOfYourPot
  min: MinimumIntValue
```

Of course, you can do this by hand in your MySQL server, but it's just there for
convenience. Note: Obviously, you should NOT expose this to the outside world.

After that, all you need to do is to issue GET requests to

```
  http://yourserver/id/NameOfYourPot
```

and a text/plain response with just the ID in its body will be returned, which
you can be sure that it will not be generated again from that same pot (as long
as you don't manually muck witht he backend DB)

Building
========

```
  go build -o idpot-server cli/idpot-server.go
```

Using idpot-server
==================

```
  $ idpot-server --config=/path/to/config.gcfg
```

The config file must at least specify the necessary information to connect to
a MySQL server

```
[Server]
LogFile = /path/to/access_log-%Y%m%d
LogLinkName = /path/to/access_log

[Mysql]
ConnectString = user:password@tcp(host:port)/dbname
```
