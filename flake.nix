{
  description = "BUAA campus network login tool";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      linuxSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
      forLinuxSystems = nixpkgs.lib.genAttrs linuxSystems;
      pkgsFor = system: nixpkgs.legacyPackages.${system};
      version = nixpkgs.lib.strings.fileContents ./VERSION;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = pkgsFor system;
          source = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./cmd
              ./pkg
              ./go.mod
              ./go.sum
            ];
          };
        in
        {
          default = pkgs.buildGoModule {
            pname = "buaa-login";
            inherit version;
            src = source;
            vendorHash = "sha256-kPBpf41hpkA0d6Zp6ZGnXfHg6ndlsP2qHxdur8Fp5MI=";
            subPackages = [ "cmd/buaa-login" ];
            checkPhase = ''
              runHook preCheck
              go test ./...
              runHook postCheck
            '';
            ldflags = [
              "-s"
              "-w"
              "-X main.Version=v${version}"
            ];
            meta = {
              description = "BUAA campus network login tool";
              homepage = "https://github.com/tsx8/buaa-login";
              license = pkgs.lib.licenses.mit;
              mainProgram = "buaa-login";
              platforms = supportedSystems;
            };
          };
        }
      );

      formatter = forAllSystems (system: (pkgsFor system).nixfmt);

      devShells = forAllSystems (
        system:
        let
          pkgs = pkgsFor system;
        in
        {
          default = pkgs.mkShellNoCC {
            packages = [
              pkgs.actionlint
              pkgs.go
              pkgs.nixfmt
              pkgs.statix
            ];
          };
        }
      );

      nixosModules.default =
        { lib, pkgs, ... }:
        {
          imports = [ ./module.nix ];
          services.buaa-login.package =
            lib.mkDefault
              self.packages.${pkgs.stdenv.hostPlatform.system}.default;
        };

      checks = forLinuxSystems (
        system:
        let
          inherit (nixpkgs) lib;
          pkgs = pkgsFor system;
          package = self.packages.${system}.default;
          storeCredentials = pkgs.writeText "buaa-login-test-credentials.json" (
            builtins.toJSON {
              stuid = "test-user";
              paswd = "test-password";
            }
          );
          mockPackage = pkgs.writeShellApplication {
            name = "buaa-login";
            runtimeInputs = [ pkgs.jq ];
            text = ''
              test "$#" -eq 6
              test "$1" = "--credentials-file"
              test "$3" = "-r"
              test "$4" = "3"
              test "$5" = "--interface"
              test "$6" = "eth0"
              test "$(jq -r .stuid "$2")" = "test-user"
              test "$(jq -r .paswd "$2")" = "test-password"
            '';
          };
          evaluatedModule = nixpkgs.lib.nixosSystem {
            inherit system;
            modules = [
              self.nixosModules.default
              {
                services.buaa-login.credentialsFile = "/run/secrets/buaa-login.json";
              }
            ];
          };
          invalidCredentialsModule = lib.evalModules {
            specialArgs = { inherit pkgs; };
            modules = [
              ./module.nix
              {
                options = {
                  assertions = lib.mkOption {
                    type = lib.types.listOf lib.types.unspecified;
                    default = [ ];
                  };
                  systemd = lib.mkOption {
                    type = lib.types.raw;
                    default = { };
                  };
                };
                config.services.buaa-login = {
                  enable = true;
                  package = mockPackage;
                  credentialsFile = toString storeCredentials;
                };
              }
            ];
          };
          storePathAssertion = builtins.elemAt invalidCredentialsModule.config.assertions 1;
        in
        {
          inherit package;

          actionlint =
            pkgs.runCommand "buaa-login-actionlint"
              {
                nativeBuildInputs = [ pkgs.actionlint ];
              }
              ''
                actionlint \
                  ${./.github/workflows/build.yml} \
                  ${./.github/workflows/ci.yml} \
                  ${./.github/workflows/nix.yml} \
                  ${./.github/workflows/publish.yml}
                touch "$out"
              '';

          formatting =
            pkgs.runCommand "buaa-login-nix-formatting"
              {
                nativeBuildInputs = [ pkgs.nixfmt ];
              }
              ''
                nixfmt --check ${./flake.nix} ${./module.nix}
                touch "$out"
              '';

          statix =
            pkgs.runCommand "buaa-login-statix"
              {
                nativeBuildInputs = [ pkgs.statix ];
              }
              ''
                statix check ${./.}
                touch "$out"
              '';

          module-evaluation =
            assert !storePathAssertion.assertion;
            pkgs.runCommand "buaa-login-module-evaluation" { } ''
              test ${pkgs.lib.escapeShellArg (toString evaluatedModule.config.services.buaa-login.package)} = \
                ${pkgs.lib.escapeShellArg (toString package)}
              touch "$out"
            '';

          vm-test = pkgs.testers.runNixOSTest {
            name = "buaa-login-module-test";
            nodes.machine = {
              imports = [ self.nixosModules.default ];

              services.lvm.enable = false;
              documentation.enable = false;

              services.buaa-login = {
                enable = true;
                package = mockPackage;
                credentialsFile = "/run/credentials/buaa-login.json";
                interface = "eth0";
                interval = "1h";
              };

              systemd.timers.buaa-login.timerConfig.OnBootSec = pkgs.lib.mkForce "1d";
            };

            testScript = ''
              machine.wait_for_unit("multi-user.target")
              machine.succeed("systemctl is-enabled buaa-login.timer")
              machine.succeed("systemctl cat buaa-login.service | grep -F 'LoadCredential=buaa-login:'")
              machine.succeed("systemctl cat buaa-login.service | grep -F 'StartLimitIntervalSec=0'")
              machine.succeed("systemctl cat buaa-login.service | grep -F 'RestartPreventExitStatus=2 3'")
              machine.succeed("systemctl show buaa-login.service -p RestartUSec --value | grep -Fx 30s")
              machine.succeed("systemctl show buaa-login.service -p DynamicUser --value | grep -Fx yes")
              machine.succeed("install -d -m 0700 /run/credentials")
              machine.succeed("printf '%s' '{\"stuid\":\"test-user\",\"paswd\":\"test-password\"}' > /run/credentials/buaa-login.json")
              machine.succeed("chmod 0600 /run/credentials/buaa-login.json")
              machine.succeed("systemctl start buaa-login.service")
              machine.succeed("systemctl show buaa-login.service -p Result --value | grep -Fx success")
              machine.succeed("systemctl cat buaa-login-resume.service | grep -F 'ExecStop='")
              machine.succeed("${package}/bin/buaa-login --help")
            '';
          };
        }
      );
    };
}
