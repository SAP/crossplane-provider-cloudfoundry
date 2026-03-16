{ lib }:
{
  exporter-cli = {
    name = "xpcf";
    version = "0.0.1-alpha1";
    vendorHash = "sha256-9JX69YJXrDAXoaBxITvkBTMLr/nlLYGEG1DV4Vswu8Q=";
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
