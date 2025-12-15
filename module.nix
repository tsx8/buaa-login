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
        Path to a file containing credentials.
        The file format must be: `<Student ID> <Password>`
        (Separated by a space).
        
        Example file content:
        23371263 MySuperSecretPassword
      '';
    };

    stuid = mkOption {
      type = types.nullOr types.str;
      description = "Student ID for login.";
      example = "23371263";
    };

    stupwd = mkOption {
      type = types.nullOr types.str;
      default = null;
      description = "Password (fallback if configFile is not set). UNSAFE: Store in world-readable store.";
    };
  };

  config = mkIf cfg.enable {
    assertions = [
      {
        assertion = (cfg.configFile != null) || (cfg.stuid != null && cfg.stupwd != null);
        message = "services.buaa-login: You must set either 'configFile' (recommended) or both 'stuid' and 'stupwd'.";
      }
    ];

    systemd.services.buaa-login = {
      description = "BUAA Campus Network Auto Login";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = if cfg.interval == null then [ "multi-user.target" ] else [];

      startLimitIntervalSec = 0;

      serviceConfig = {
        Type = "oneshot";
        Restart = if cfg.interval != null then "no" else "on-failure";
        RestartSec = "10s";
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

          exec ${cfg.package}/bin/buaa-login -i "$USER_ID" -p "$USER_PWD"
        '';
      };

      systemd.timers.buaa-login = mkIf (cfg.interval != null) {
        description = "Periodic Timer for BUAA Campus Network Login";
        wantedBy = [ "timers.target" ];
        timerConfig = {
          OnBootSec = "1m";
          OnUnitActiveSec = cfg.interval;
          Unit = "buaa-login.service";
        };
      };
    };
  };
}