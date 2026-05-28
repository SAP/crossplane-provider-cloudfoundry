{ lib }:
{
  exporter-cli = {
    name = "xpcf";
    version = "0.0.1-alpha2";
    vendorHash = "sha256-Dzk73QrvdOH/dWqNTi5kCjau3nNqx3w99/SfAqnakEY=";
    # vendorHash = lib.fakeHash;
    meta = {
      description = "xpcf is a CLI tool for exporting existing resources as Crossplane managed resources";
      homepage = "https://github.com/SAP/crossplane-provider-cloudfoundry";
      license = lib.licenses.asl20;
    };
    src = with lib; fileset.toSource {
      root = ./.;
      fileset = fileset.unions [
        (fileset.fromSource (sources.sourceFilesBySuffices ./. [".go"]))
        ./go.mod
        ./go.sum
        ./flake.nix
        ./flake.lock
        ./config.nix
      ];
    };
  };
}
