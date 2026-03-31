{
  description = "Ratchet — Debate-driven quality plugin for Claude Code";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        ratchet = pkgs.stdenvNoCC.mkDerivation {
          pname = "ratchet";
          version = "0.1.0";
          src = ./.;

          nativeBuildInputs = [ pkgs.makeWrapper ];

          installPhase = ''
            mkdir -p $out/share/ratchet
            cp -r agents scripts skills schemas $out/share/ratchet/
            cp install.sh $out/share/ratchet/

            mkdir -p $out/bin
            makeWrapper $out/share/ratchet/install.sh $out/bin/ratchet-install \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.git pkgs.coreutils pkgs.gnused pkgs.yq-go ]}
          '';
        };
      in
      {
        packages = {
          default = ratchet;
          ratchet = ratchet;
        };

        apps.default = {
          type = "app";
          program = "${ratchet}/bin/ratchet-install";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            shellcheck
            jq
            yq-go
            git
            bash
            go
            golangci-lint
            check-jsonschema
          ];
          shellHook = ''
            echo "Ratchet development environment loaded"
            echo "Available tools: shellcheck, jq, yq, git, go, golangci-lint"
          '';
        };
      }
    );
}
