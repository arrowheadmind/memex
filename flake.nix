{
  description = "memex: a command-line tool for a private, git-backed store of Markdown notes";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAll = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      # A flake cannot see git tags, so the version is set here: a release commit sets it to
      # the tag's version (IMPLEMENTATION.md, "Packaging"); every other build is 0.0.0-dev.
      version = "0.0.0-dev";
    in
    {
      packages = forAll (pkgs: rec {
        memex = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
          pname = "memex";
          inherit version;
          src = self;
          subPackages = [ "cmd/memex" ];
          vendorHash = "sha256-yasi85AngN74XaZtympttLt6rNWc7AZaTgMCCP8F5pU=";
          env.CGO_ENABLED = "0";
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];
          # The unit tests run here; the conformance suite needs git and runs in scripts/check.sh.
          nativeCheckInputs = [ pkgs.git ];
          meta = {
            description = "Command-line tool for a private, git-backed store of Markdown notes";
            mainProgram = "memex";
            license = pkgs.lib.licenses.mit;
          };
        };
        default = memex;
      });
    };
}
