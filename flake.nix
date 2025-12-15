{
  description = "BUAA Campus Network Login Tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      nixpkgsFor = system: nixpkgs.legacyPackages.${system};
      version = nixpkgs.lib.strings.fileContents ./VERSION;
    in
    {
      packages = forAllSystems (system:
        let pkgs = nixpkgsFor system; in {
          default = pkgs.buildGoModule {
            pname = "buaa-login";
            inherit version;
            src = ./.;
            vendorHash = null;
            subPackages = [ "cmd/buaa-login" ];
            ldflags = [ 
              "-s" "-w" 
              "-X main.Version=v${version}" 
            ];
            meta = with pkgs.lib; {
              description = "BUAA Campus Network Login Tool";
              license = licenses.mit;
              mainProgram = "buaa-login";
            };
          };
        });

      nixosModules.default = { config, lib, pkgs, ... }: 
        let
          overlay = final: prev: {
            buaa-login = self.packages.${prev.system}.default;
          };
        in {
          nixpkgs.overlays = [ overlay ];
          imports = [ ./module.nix ];
        };
    };
}