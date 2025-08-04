mod cluster;

use crate::cluster::docker::DockerInstance;
use crate::cluster::{Instance, docker};
use bollard::Docker;
use bollard::query_parameters::ListContainersOptions;
use clap::{Arg, Command};
use std::default::Default;
use std::vec::IntoIter;
use tokio_postgres::{Error, NoTls};

#[tokio::main]
async fn main() -> Result<(), anyhow::Error> {
    let init_command = Command::new("init")
        .about("Initialize a new Omnigres project")
        .arg(
            Arg::new("path")
                .help("The path to initialize the project in")
                .default_value("."),
        )
        .arg(
            Arg::new("name")
                .long("name")
                .help("The name of the project"),
        );
    let command = Command::new("omnigres")
        .bin_name("omnigres")
        .subcommand(init_command)
        .about("A command line interface for Omnigres");

    /*
    let rows = client
        .query("select * from omni_cli.cli_command", &[])
        .await?;

    let command = rows.into_iter().fold( Command::new("omnigres"), |mut root, row| {
        let name: String = row.get("name");
        // Clap insists on a statically allocated name
        let static_name: &'static str = Box::leak(name.into_boxed_str());
        let description: String = row.get("description");

        let cmd = clap::Command::new(static_name).about(description);
        root.subcommand(cmd)
    }).bin_name("omnigres");


    command.get_matches();
     */

    let m = command.get_matches();
    println!("{:?}", m);

    for instance in docker_instances().await? {
        println!("--- {} {}", instance.name().await?, instance.postgres_connection_string().await?);
        let (client, connection) =
            tokio_postgres::connect(&container.postgres_connection_string().await?, NoTls).await?;

        tokio::spawn(async move {
            if let Err(e) = connection.await {
                eprintln!("connection error: {}", e);
            }
        });
    }
    /*


        let rows = client
            .query("select  1", &[])
            .await?;

    }*/

    Ok(())
}

async fn docker_instances() -> Result<Vec<DockerInstance>, anyhow::Error> {
    let docker = Docker::connect_with_defaults()?;
    let containers = docker::DockerHost::new(&docker).list().await?;
    Ok(containers)
}
