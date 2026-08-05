{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.buaa-login;
in
{
  options.services.buaa-login = {
    enable = lib.mkEnableOption "BUAA campus network automatic login";

    package = lib.mkOption {
      type = lib.types.package;
      description = "The buaa-login package to run.";
    };

    credentialsFile = lib.mkOption {
      type = lib.types.str;
      example = "/run/secrets/buaa-login.json";
      description = ''
        Absolute runtime path to a JSON file containing the stuid and paswd fields.
        Keep this file outside the Nix store; it is passed to the service through
        systemd's credential mechanism.
      '';
    };

    interval = lib.mkOption {
      type = lib.types.nullOr lib.types.str;
      default = null;
      example = "15min";
      description = ''
        Interval between successful login checks. When null, the service starts
        at boot and relies on systemd restart handling after failures.
      '';
    };

    wakeUp = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Whether to start a login attempt after the system resumes from sleep.";
    };

    retry = lib.mkOption {
      type = lib.types.ints.unsigned;
      default = 3;
      description = "Number of retries performed by buaa-login after the initial attempt.";
    };
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = lib.hasPrefix "/" cfg.credentialsFile;
        message = "services.buaa-login.credentialsFile must be an absolute runtime path.";
      }
      {
        assertion = cfg.interval == null || cfg.interval != "";
        message = "services.buaa-login.interval must be null or a non-empty systemd time span.";
      }
    ];

    systemd = {
      services.buaa-login = {
        description = "BUAA campus network automatic login";
        after = [ "network-online.target" ];
        wants = [ "network-online.target" ];
        wantedBy = lib.optionals (cfg.interval == null) [ "multi-user.target" ];

        startLimitIntervalSec = 60;
        startLimitBurst = 5;

        serviceConfig = {
          Type = "oneshot";
          Restart = "on-failure";
          RestartSec = "5s";
          DynamicUser = true;

          LoadCredential = "buaa-login:${cfg.credentialsFile}";
          ExecStart = "${lib.getExe cfg.package} --credentials-file %d/buaa-login -r ${toString cfg.retry}";

          CapabilityBoundingSet = "";
          LockPersonality = true;
          MemoryDenyWriteExecute = true;
          NoNewPrivileges = true;
          PrivateDevices = true;
          PrivateTmp = true;
          ProtectClock = true;
          ProtectControlGroups = true;
          ProtectHome = true;
          ProtectHostname = true;
          ProtectKernelLogs = true;
          ProtectKernelModules = true;
          ProtectKernelTunables = true;
          ProtectSystem = "strict";
          RestrictAddressFamilies = [
            "AF_INET"
            "AF_INET6"
          ];
          RestrictNamespaces = true;
          RestrictRealtime = true;
          RestrictSUIDSGID = true;
          SystemCallArchitectures = "native";
          UMask = "0077";
        };
      };

      timers.buaa-login = lib.mkIf (cfg.interval != null) {
        description = "Periodic BUAA campus network login timer";
        wantedBy = [ "timers.target" ];
        timerConfig = {
          OnBootSec = "30s";
          OnUnitInactiveSec = cfg.interval;
          Unit = "buaa-login.service";
        };
      };

      services.buaa-login-resume = lib.mkIf cfg.wakeUp {
        description = "Trigger BUAA campus network login after resume";
        wantedBy = [ "sleep.target" ];
        before = [ "sleep.target" ];
        unitConfig = {
          DefaultDependencies = false;
          StopWhenUnneeded = true;
        };
        serviceConfig = {
          Type = "oneshot";
          RemainAfterExit = true;
          ExecStart = "${pkgs.coreutils}/bin/true";
          ExecStop = "${pkgs.systemd}/bin/systemctl --no-block start buaa-login.service";
        };
      };
    };
  };
}
