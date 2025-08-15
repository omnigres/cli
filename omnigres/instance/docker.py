from docker import DockerClient
from docker.models.containers import Container

from .instance import Instance


class DockerInstance(Instance):
    @classmethod
    def list(cls):
        client = DockerClient.from_env()
        client.containers.list()
        instances = [DockerInstance(client, container) for container in client.containers.list()]
        return [instance for instance in instances if instance.valid()]

    def __init__(self, client: DockerClient, container: Container):
        super().__init__()
        self.client = client
        self.container = container

    def __repr__(self):
        return f"<DockerInstance {self.name()}>"

    def valid(self) -> bool:
        binding = next(iter(self._port_bindings()), None)
        if binding is None:
            return False
        matching_label = self.container.image.labels.get("org.opencontainers.image.title") == "omnigres"
        matching_entrypoint = self.container.attrs.get("Config", {}).get("Entrypoint", "") == ["omnigres-entrypoint.sh"]
        return matching_label or matching_entrypoint

    def name(self) -> str:
        return self.container.name

    def identifier(self) -> str:
        return self.container.id

    def postgres_connection_string(self, dbname: str = None) -> str:
        binding = next(iter(self._port_bindings()), None)
        if binding:
            return f"dbname={dbname or "omnigres"} user=omnigres password=omnigres host={binding.get('HostIp')} port={binding.get('HostPort')}"

    def _port_bindings(self):
        return (self.container.attrs.get("HostConfig", {}).get("PortBindings", {}) or {}).get("5432/tcp", [])
