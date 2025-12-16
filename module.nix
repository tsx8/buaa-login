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
      description = ''
        The buaa-login package to use.
      '';
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

    wakeUp = mkEnableOption "Run login check on wake up from sleep/suspend" // {
      default = true;
    };

    retry = mkOption {
      type = types.int;
      default = 3;
      description = "Max retry times on login failure.";
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
      after = [ "network-online.target" "sleep.target" ];
      wants = [ "network-online.target" ];
      wantedBy = 
        let
          bootTargets = if cfg.interval == null then [ "multi-user.target" ] else [];
          wakeUpTargets = if cfg.wakeUp then [ "sleep.target" ] else [];
        in
          bootTargets ++ wakeUpTargets;

      startLimitIntervalSec = 60;
      startLimitBurst = 5;

      restartIfChanged = false;

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

          exec ${cfg.package}/bin/buaa-login -i "$USER_ID" -p "$USER_PWD" -r ${toString cfg.retry}
        '';
      };
    };
    systemd.timers.buaa-login = mkIf (cfg.interval != null) {
      description = "Periodic Timer for BUAA Login";
      wantedBy = [ "timers.target" ];
      timerConfig = {
        OnBootSec = "1s";
        AccuracySec = "1s";
        OnUnitActiveSec = cfg.interval;
        Unit = "buaa-login.service";
      };
    };
  };
}
