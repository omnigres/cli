# {{ cookiecutter.name }}

{{ cookiecutter.description }}

## Operations

### Starting

In the project root, run:

```shell
omnigres start
```

Starts an Omnigres instance. If no instance exists, one will be created automatically – preferring Docker when available, otherwise falling back to other supported environments. Creates a default workspace at .omnigres/instance.db.

