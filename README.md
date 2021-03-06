![Build status](https://github.com/function61/pdfrasterizer/workflows/Build/badge.svg)

What?
-----

A small microservice that rasterizes a PDF as an image file (use case: thumbnailing).

You can run this:

- on AWS Lambda
- with Docker
  * I didn't bother making a `Dockerfile` though, since I didn't need it. PR welcome!
- as a standalone binary

There also exists [a small client library for Go](pkg/pdfrasterizerclient/)


Testing
-------

You can start a local server process with:

```console
$ pdfrasterizer server
```

Then call it from the client:

```console
$ export PDFRASTERIZER_TOKEN="doesntMatter" # optionally you can put the service behind authentication
$ pdfrasterizer client-localhost example.pdf > example.png
```

You can also use raw curl:

```console
$ curl -H 'Content-Type: application/pdf' -X POST --data-binary @example.pdf http://localhost/rasterize > example.png
```


Internals
---------

Internally, it uses [Ghostscript](https://www.ghostscript.com/).
