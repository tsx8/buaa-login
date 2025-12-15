{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.buaa-login;
in
{
  options.services.buaa-login = {
    enable = mkEnableOption "BUAA Campus Network Auto Login Service";

    package = mkOption {
      type = types.package;
      default = pkgs.buaa-login;
      description = "The buaa-login package to use.";
    };

    interval = mkOption {
      type = types.nullOr types.str;
      default = null;
      example = "15min";
      description = ''
        Time interval for periodic login checks.
        If set (e.g., "15min", "1h"), a systemd timer will trigger the login attempt periodically.
        If null, the service runs in 'keep-alive' mode (restarts immediately on failure).
      '';
    };

    configFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = ''
        Path to a file containing credentials (format: `<ID> <PWD>`).
      '';
    };

    stuid = mkOption {
      type = types.nullOr types.str;
      default = null;
      description = "Student ID.";
    };

    stupwd = mkOption {
      type = types.nullOr types.str;
      default = null;
      description = "Password.";
    };
  };

  config = mkIf cfg.enable {
    assertions = [
      {
        assertion = (cfg.configFile != null) || (cfg.stuid != null && cfg.stupwd != null);
        message = "services.buaa-login: Set 'configFile' or both 'stuid' and 'stupwd'.";
      }
    ];

    systemd.services.buaa-login = {
      description = "BUAA Campus Network Auto Login";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = if cfg.interval == null then [ "multi-user.target" ] else [];

      startLimitIntervalSec = 60;
      startLimitBurst = 5;

      serviceConfig = {
        Type = "oneshot";
        Restart = "on-failure";
        RestartSec = "5s";
        User = "root";  
        
        ExecStart = pkgs.writeShellScript "buaa-login-start" ''
          if [ -n "${toString cfg.configFile}" ]; then
            if [ -f "${toString cfg.configFile}" ]; then
              read -r USER_ID USER_PWD < "${toString cfg.configFile}"
            else
              echo "Error: Config file ${toString cfg.configFile} not found!"
              exit 1
            fi
          else
            USER_ID="${toString cfg.stuid}"
            USER_PWD="${toString cfg.stupwd}"
          fi

          if [ -z "$USER_ID" ] || [ -z "$USER_PWD" ]; then
             echo "Error: ID or Password is empty."
             exit 1
          fi

          exec ${cfg.package}/bin/buaa-login -i "$USER_ID" -p "$USER_PWD" -r 0
        '';
      };
    };
    systemd.timers.buaa-login = mkIf (cfg.interval != null) {
      description = "Periodic Timer for BUAA Login";
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnBootSec = "1m";
        OnUnitActiveSec = cfg.interval;
        Unit = "buaa-login.service";
      };
    };
  };
}