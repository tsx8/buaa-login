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

      checks = forLinuxSystems (
        system:
        let
          pkgs = nixpkgsFor system;
        in
        {
          vm-test = pkgs.testers.runNixOSTest {
            name = "buaa-login-test";
            nodes.machine =
              { config, pkgs, ... }:
              {
                imports = [ ./module.nix ];

                services.lvm.enable = false;
                documentation.enable = false;

                services.buaa-login = {
                  enable = true;
                  package = self.packages.${system}.default;
                  stuid = "test-user";
                  stupwd = "test-password";
                  interval = "1h";
                };

                # Delay to 1 day, won't be triggered during test
                systemd.timers.buaa-login.timerConfig = {
                  OnBootSec = pkgs.lib.mkForce "1d";
                };
              };

            testScript =
              let
                buaaLoginBin = "${self.packages.${system}.default}/bin/buaa-login";
              in
              ''
                machine.wait_for_unit("multi-user.target")
                machine.succeed("systemctl cat buaa-login.service")
                machine.succeed("systemctl cat buaa-login.timer")
                machine.succeed("systemctl is-enabled buaa-login.timer")
                machine.succeed("test -x ${buaaLoginBin}")
                machine.succeed("${buaaLoginBin} --help")
                machine.succeed("systemctl is-active buaa-login.service || true")
              '';
          };
        }
      );
    };
}
