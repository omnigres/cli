# Omnigres CLI

This is a Command Line Interface to interact with [Omnigres](https://github.com/omnigres/omnigres).
Also check the [Omnigres documentation](https://docs.omnigres.org/) for more info about the platform.

## Quick start

```sh 
curl --proto '=https' --tlsv1.2 -sSf https://get.omnigres.org | sh
```

Alternatively you can download binaries from the [releases page](/releases) for your architecture and place within your **PATH**. 

Once you have the `og` executable available go ahead and try to start a transient cluster, all state will be erased at the end.

```sh 
og run
```

If everything is working at the end of the cluster creation you should see something like the output below.

```sh 
╭──────────────────────────────────────────────────────────────────────────────╮
│Omnigres Cluster                                                              │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│omnigres (Postgres): postgres://omnigres:omnigres@172.19.0.2:5432/omnigres    │
│omnigres (HTTP): http://172.19.0.2:8080                                       │
╰──────────────────────────────────────────────────────────────────────────────╯
```

Omnigres can serve HTTP requests out of the box using the [omni_httpd](https://docs.omnigres.org/omni_httpd/install/) extension.
Verify the HTTP connection on your browser and you should see the default dashboard running.
Hit `Ctrl+C` and the cluster will be terminated and all state erased.

### Your first HTTP application

To create an application from scratch start using the init command.

```sh 
og init my-first-app
```

This will create a `my-first-app/src` directory. We can place a python source file there to create a Flask application.
Place the example below inside `my-first-app/src/hello.py`

```python
from omni_python import pg
from flask import Flask
from omni_http.omni_httpd import flask

app = Flask('myapp')


@app.route('/')
def hello():
    return "Hello, world!"


handle = pg(flask.Adapter(app))
```

Now run the server with `og run`. Once you open the HTTP address on the web browser the message "Hello, world!" will be rendered.

## Installing from source

> TBD...
> here we can have the old quick start section with a few more details since it will be geared mostly for developers
