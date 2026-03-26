fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Ensure Cargo reruns build.rs when proto files change.
    // Proto files live outside the crate directory, so Cargo won't detect changes
    // without explicit rerun-if-changed directives.
    println!("cargo:rerun-if-changed=../proto/hive/v1/types.proto");
    println!("cargo:rerun-if-changed=../proto/hive/v1/api.proto");
    println!("cargo:rerun-if-changed=../proto/hive/v1/mesh.proto");

    tonic_build::configure()
        .build_server(false) // CLI only needs the client
        .compile_protos(
            &["../proto/hive/v1/types.proto", "../proto/hive/v1/api.proto"],
            &["../proto"],
        )?;
    Ok(())
}
