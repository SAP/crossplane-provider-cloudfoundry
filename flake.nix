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
      perSystem = { config, self', inputs', pkgs, lib, system, ... }: {
        packages = rec {
          exporter = pkgs.buildGoModule {
            pname = "xpcf";
            version = "0.0.1-alpha1";
            ldflags = ["-X main.ShortName=xpcf"];
            src = ./.;
            subPackages = ["cmd/exporter"];
            vendorHash = "sha256-HiWXSvLwRzt1/wMl2LQfwrBPoRpsl5E3TsN/1N0PGWs=";
            # vendorHash = lib.fakeHash;
          };
          xpcf = pkgs.runCommand "xpcf" {} ''
              mkdir -p $out/bin
              cp ${exporter}/bin/exporter $out/bin/xpcf
            '';
        };
        apps.exporter = {
          type = "app";
          program = "${self'.packages.xpcf}/bin/xpcf";
        };
      };
      flake = {};
    };
}
