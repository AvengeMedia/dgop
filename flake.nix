{
  description = "API & CLI for System Resources";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];

      forAllSystems =
        f:
        builtins.listToAttrs (
          map (system: {
            name = system;
            value = f system;
          }) supportedSystems
        );

    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          inherit (pkgs) lib;
        in
        {
          dgop = pkgs.buildGoModule (finalAttrs: {
            pname = "dgop";
            version = "0.1.11";
            src = ./.;
            vendorHash = "sha256-OxcSnBIDwbPbsXRHDML/Yaxcc5caoKMIDVHLFXaoSsc=";

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${finalAttrs.version}"
              "-X main.buildTime=1970-01-01_00:00:00"
              "-X main.Commit=${finalAttrs.version}"
            ];

            nativeBuildInputs = with pkgs; [
              installShellFiles
              makeWrapper
            ];

            installPhase = ''
              mkdir -p $out/bin
              cp $GOPATH/bin/dgop $out/bin/dgop
              wrapProgram $out/bin/dgop --prefix PATH : "${lib.makeBinPath [ pkgs.pciutils ]}"

              installShellCompletion --cmd dgop \
                  --bash <($out/bin/dgop completion bash) \
                  --fish <($out/bin/dgop completion fish) \
                  --zsh <($out/bin/dgop completion zsh)
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

          default = self.packages.${system}.dgop;
        }
      );
    };
}
