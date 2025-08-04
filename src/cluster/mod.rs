pub mod docker;

use std::error::Error;
pub trait Instance<E> {
    type Error: Into<E>;
    async fn name(&self) -> Result<String, Self::Error>;
    async fn postgres_connection_string(&self) -> Result<String, Self::Error>;
}

/*
pub trait Cluster {
    type Error : Error;
    type Node : Node;

    async fn init(&self) -> Result<Self::Node, Self::Error>;
}
 */
