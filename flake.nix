{
  description = "Description for the project";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = inputs@{ flake-parts, ... }:
    let
      exporter-cli = {
        name = "xpcf";
        version = "0.0.1-alpha1";
        meta = lib: {
          description = "xpcf is a CLI tool for exporting existing resources as Crossplane managed resources";
          homepage = "https://github.com/SAP/crossplane-provider-cloudfoundry";
          license = lib.licenses.asl20;
        };
        src = lib: with lib; fileset.toSource {
          root = ./.;
          fileset = fileset.unions [
            (fileset.fromSource (sources.sourceFilesBySuffices ./. [".go"]))
            ./go.mod
            ./go.sum
          ];
        };

      };
    in
      flake-parts.lib.mkFlake { inherit inputs; } {
        imports = [
        ];
        systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
        perSystem = { config, self', inputs', pkgs, lib, system, ... }: {
          packages = rec {
            exporter = pkgs.buildGoModule {
              inherit (exporter-cli) version;
              pname = exporter-cli.name;
              ldflags = ["-X main.ShortName=${exporter-cli.name}"];
              src = exporter-cli.src lib;
              subPackages = ["cmd/exporter"];
              vendorHash = "sha256-HiWXSvLwRzt1/wMl2LQfwrBPoRpsl5E3TsN/1N0PGWs=";
              meta = exporter-cli.meta lib;
            };
            "${exporter-cli.name}" = pkgs.runCommand "${exporter-cli.name}" {} ''
              mkdir -p $out/bin
              cp ${exporter}/bin/exporter $out/bin/${exporter-cli.name}
            '';
          };
          devShells.default = pkgs.mkShell {
            packages = with pkgs; [go];
          };
          apps.exporter = {
            meta = exporter-cli.meta lib;
            type = "app";
            program = "${self'.packages.${exporter-cli.name}}/bin/${exporter-cli.name}";
          };
          checks = {
            exporter = pkgs.runCommand "exporter-help" {} ''
                     ${self'.packages.${exporter-cli.name}}/bin/${exporter-cli.name} --help > $out
            '';
          };
        };
        flake = {};
      };
}
