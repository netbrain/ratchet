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
            cp -r .claude-plugin agents hooks scripts skills $out/share/ratchet/
            cp install.sh $out/share/ratchet/

            mkdir -p $out/bin
            makeWrapper $out/share/ratchet/install.sh $out/bin/ratchet-install \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.python3 pkgs.git pkgs.coreutils pkgs.gnused ]}
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
      }
    );
}
