# Omnigres CLI

This is a Command Line Interface to interact with [Omnigres](https://github.com/omnigres/omnigres).
Also check the [Omnigres documentation](https://docs.omnigres.org/) for more info about the platform.

## Before you start

In order to provision an Omnigres cluster you should have the Docker CLI installed.
Check their [Get Started page](https://www.docker.com/get-started/) to install the Docker tools on your system.

## Quick start

Download binaries from the [releases page](/releases) for your architecture and place within your **PATH**. Then try calling it without parameters

```sh
omnigres
```

This will show a list of all available commands.

### Your first HTTP application

To create an application from scratch start using the init command. But we should start in a new root directory for your Omnigres projects.

```sh 
mkdir og_projects
cd og_projects
omnigres init first_app
```

Now run the server with `omnigres start`. 

```sh
omnigres start
```  

You should see all the available endpoints once the command finishes.

```sh
INFO Omnigres Orb cluster started.
omnigres (Postgres): postgres://omnigres:omnigres@172.19.0.3:5432/omnigres
omnigres (HTTP): http://172.19.0.3:8081
```

You should have now a `first_app/src` directory. We can place a little function and router there.
Just copy the contents below into `first_app/src/hello.sql`.

```sql
create extension omni_httpd cascade;

create function my_handler(request omni_httpd.http_request)
  returns omni_httpd.http_outcome
  return omni_httpd.http_response(body => 'Hello World');

create table my_router (like omni_httpd.urlpattern_router);

insert into my_router (match, handler)
values (omni_httpd.urlpattern('/'), 'my_handler'::regproc);
```

After saving the file you can create the application using the command `assemble`.
This will build a new database (also called Orb in this context) named first_app using the contents of `first_app/src`.


```sh
omnigres assemble
```
Now check your current endpoints using the `endpoints` command. 

```sh
omnigres endpoints
```

The result should be similar to the one below.
Note that the IP addresses and ports might differ.

```sh
omnigres (Postgres): postgres://omnigres:omnigres@172.19.0.3:5432/omnigres
omnigres (HTTP): http://172.19.0.3:8081
first_app (Postgres): postgres://omnigres:omnigres@172.19.0.3:5432/first_app
first_app (HTTP): http://172.19.0.3:8080
```

Open the HTTP address in your browser for `first_app`, and you should see the message 'Hello, world!'

### Stopping the service

To stop the service just use the `stop` command.

```sh
omnigres stop
```

## Compiling from source

First ensure you have a working [go development environment](https://go.dev/dl/).

```sh 
go build -o omnigres cmd/omnigres/main.go
```
The command above should compile a binary `omnigres` at the project's root folder.
