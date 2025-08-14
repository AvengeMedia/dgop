{
  description = "API & CLI for System Resources";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      perSystem =
        {
          self',
          pkgs,
          ...
        }:
        let
          lib = pkgs.lib;
        in
        {
          packages = {
            default = self'.packages.dgop;
            dgop = pkgs.buildGoModule (finalAttrs: {
              pname = "dgop";
              version = "0.0.6";
              src = ./.;
              vendorHash = "sha256-+5rN3ekzExcnFdxK2xqOzgYiUzxbJtODHGd4HVq6hqk=";
              ldflags = [
                "-s"
                "-w"
                "-X main.Version=${finalAttrs.version}"
                "-X main.buildTime=1970-01-01_00:00:00"
                "-X main.Commit=${finalAttrs.version}"
              ];

              installPhase = ''
                mkdir -p $out/bin
                cp $GOPATH/bin/cli $out/bin/dgop
              '';

              meta = {
                description = "API & CLI for System Resources";
                homepage = "https://github.com/AvengeMedia/dgop";
                mainProgram = "dgop";
                binaryNativeCode = true;
                license = lib.licenses.mit;
                platforms = lib.platforms.unix;
                maintainers = with lib.maintainers; [ lonerOrz ];
              };
            });
          };
        };
    };
}
