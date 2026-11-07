{
  description = "A Nix-flake-based Go development environment, for apps configured in Nickel";

  inputs.nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1"; # unstable Nixpkgs

  outputs =
    { self, ... }@inputs:

    let
      goVersion = 27; # Change this to update the whole stack

      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forEachSupportedSystem =
        f:
        inputs.nixpkgs.lib.genAttrs supportedSystems (
          system:
          f {
            inherit system;
            pkgs = import inputs.nixpkgs {
              inherit system;
              overlays = [ inputs.self.overlays.default ];
            };
          }
        );
    in
    {
      overlays.default = final: prev: {
        go = final."go_1_${toString goVersion}";
      };

      devShells = forEachSupportedSystem (
        { pkgs, system }:
        {
          # mkShell, not mkShellNoCC: go-nickel links Nickel's interpreter (a
          # static library it ships) through cgo, which needs a C compiler
          default = pkgs.mkShell {
            packages = with pkgs; [
              # go (version is specified by overlay)
              go

              # goimports, godoc, etc.
              gotools

              # https://github.com/golangci/golangci-lint
              golangci-lint

              # For writing the configuration: Nickel itself (nickel format,
              # nickel typecheck) and its language server
              nickel
              nls

              self.formatter.${system}
            ];

            env.CGO_ENABLED = "1";
          };
        }
      );

      formatter = forEachSupportedSystem ({ pkgs, ... }: pkgs.nixfmt);
    };
}
