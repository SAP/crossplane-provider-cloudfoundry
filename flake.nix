{
  description = "Description for the project";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
      ];
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      perSystem = { config, self', inputs', pkgs, lib, system, ... }:
        let
          config = import ./config.nix { inherit lib; };
        in
          {
            packages = rec {
              exporter = pkgs.buildGoModule {
                inherit (config.exporter-cli) version;
                pname = config.exporter-cli.name;
                ldflags = ["-X main.ShortName=${config.exporter-cli.name}"];
                src = config.exporter-cli.src;
                subPackages = ["cmd/exporter"];
                vendorHash = config.exporter-cli.vendorHash;
                meta = config.exporter-cli.meta;
              };
              "${config.exporter-cli.name}" = pkgs.runCommand "${config.exporter-cli.name}" {} ''
              mkdir -p $out/bin
              cp ${exporter}/bin/exporter $out/bin/${config.exporter-cli.name}
            '';
            };
            devShells.default = pkgs.mkShell {
              packages = with pkgs; [go];
            };
            apps.exporter = {
              meta = config.exporter-cli.meta;
              type = "app";
              program = "${self'.packages.${config.exporter-cli.name}}/bin/${config.exporter-cli.name}";
            };
            checks = {
              exporter = pkgs.runCommand "exporter-help" {} ''
                     ${self'.packages.${config.exporter-cli.name}}/bin/${config.exporter-cli.name} --help > $out
            '';
            };
          };
      flake = {};
    };
}
