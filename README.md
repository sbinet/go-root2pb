go-root2pb
==========

Naive generation of ``.proto`` files from ``ROOT`` flat n-tuple.

Installation
------------

```
$ go get github.com/sbinet/go-root2pb
```

Note: you'll need ``croot`` and ``go-croot`` installed.

Example
-------

```
$ go-root2pb -f ntuple.0.root -t egamma -gen=go,py
```

This will generate an ``event.proto`` file under the ``out`` directory
and generate the ``protobuf`` files for ``go`` and ``python`` for the
``ROOT::TTree`` named ``egamma``.

Limitations
-----------

The translation of ``std::vector<T>`` into their ``protobuf``
equivalent is pretty simple for the moment (just make a ``repeated``
field with the ``[packed=true]`` attribute.)

There might be issues for the cases where ``T`` is itself an
``std::vector``...

