use crate::cluster::Instance;
use bollard::Docker;
use bollard::models::ContainerSummary;
use bollard::query_parameters::{InspectContainerOptions, ListContainersOptions};
use std::fmt::format;
use std::marker::PhantomData;
use std::sync::Arc;

#[derive(Debug)]
pub struct DockerInstance<E: From<Error> = Error> {
    docker: Docker,
    name: String,
    _error: PhantomData<E>,
}

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("Docker error: {0}")]
    DockerError(#[from] bollard::errors::Error),
    #[error("No container name")]
    NoContainerName,
    #[error("No port {0} mapping")]
    NoPortMapping(&'static str),
}

impl DockerInstance {
    pub fn new(docker: &Docker, name: &str) -> Result<Self, Error> {
        Ok(Self {
            docker: docker.clone(),
            name: name.into(),
            _error: PhantomData,
        })
    }
}

impl<E: From<Error>> Instance<E> for DockerInstance<E> {
    type Error = Error;

    async fn name(&self) -> Result<String, Self::Error> {
        Ok(self.name.to_string())
    }

    async fn postgres_connection_string(&self) -> Result<String, Self::Error> {
        let inspect = self
            .docker
            .inspect_container(&self.name, None::<InspectContainerOptions>)
            .await?;
        let port_mapping = inspect
            .network_settings
            .unwrap_or_default()
            .ports
            .unwrap_or_default();
        let port = "5432/tcp";
        let pg_mapping = port_mapping
            .get(port)
            .ok_or(Error::NoPortMapping(port))?
            .as_ref()
            .ok_or(Error::NoPortMapping(port))?
            .first()
            .ok_or(Error::NoPortMapping(port))?;
        if let Some(port) = pg_mapping.host_port.as_ref() {
            Ok(format!(
                "host=localhost user=omnigres port={} dbname=omnigres password=omnigres",
                port
            ))
        } else {
            Err(Error::NoPortMapping(port))
        }
    }
}

pub struct DockerHost {
    docker: Docker,
}

impl DockerHost {
    pub fn new(docker: &Docker) -> Self {
        DockerHost {
            docker: docker.clone(),
        }
    }
    pub async fn list(&self) -> Result<Vec<DockerInstance>, Error> {
        let containers = self
            .docker
            .list_containers(None::<ListContainersOptions>)
            .await?;
        containers
            .into_iter()
            .filter(Self::is_omnigres_container)
            .map(|c| DockerInstance::new(&self.docker, c.canonical_name()?.into()))
            .collect()
    }

    fn is_omnigres_container(c: &ContainerSummary) -> bool {
        if let Some(ref labels) = c.labels {
            if let Some(label) = labels.get("org.opencontainers.image.title") {
                return label == "omnigres";
            }
        }
        false
    }
}

trait ContainerSummaryExt {
    fn canonical_name(&self) -> Result<&str, Error>;
}

impl ContainerSummaryExt for ContainerSummary {
    fn canonical_name(&self) -> Result<&str, Error> {
        Ok(self
            .names
            .as_ref()
            .ok_or(Error::NoContainerName)?
            .first()
            .ok_or(Error::NoContainerName)?
            .strip_prefix("/")
            .ok_or(Error::NoContainerName)?)
    }
}
