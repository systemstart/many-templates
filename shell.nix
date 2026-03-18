{ pkgs }:

let
  goMod = builtins.readFile ./go.mod;
  parsedFull = builtins.match ".*go ([0-9]+\.[0-9]+(\.[0-9]+)?).*" goMod;
  fullVersion = if parsedFull == null then "1.22" else builtins.elemAt parsedFull 0;
  parts = builtins.filter builtins.isString (builtins.split "\\." fullVersion);
  major = builtins.elemAt parts 0;
  minor = builtins.elemAt parts 1;
  attr = "go_${major}_${minor}";
  goPkg =
    builtins.trace "Go version from go.mod: ${fullVersion} (using nixpkgs ${attr})"
      (if pkgs ? ${attr} then pkgs.${attr} else pkgs.go);

  gsemver = let
    version = "0.10.0";
    sources = {
      "x86_64-linux" = {
        url = "https://github.com/arnaud-deprez/gsemver/releases/download/v${version}/gsemver_${version}_linux_amd64.tar.gz";
        hash = "sha256-F1oyytHMSEBZTNWVyxKM6Zua2sJeQjQ3pyyPDxYDk78=";
      };
      "aarch64-linux" = {
        url = "https://github.com/arnaud-deprez/gsemver/releases/download/v${version}/gsemver_${version}_linux_arm64.tar.gz";
        hash = "sha256-PRIp6ti87aoLoKdLWnDSJLUw+uM95olpUB2ILSmtMII=";
      };
      "x86_64-darwin" = {
        url = "https://github.com/arnaud-deprez/gsemver/releases/download/v${version}/gsemver_${version}_darwin_amd64.tar.gz";
        hash = "sha256-BBKey/Gk1gDQ3uKWuBLuPqEYdjBxxVYsBytBFOOygz4=";
      };
      "aarch64-darwin" = {
        url = "https://github.com/arnaud-deprez/gsemver/releases/download/v${version}/gsemver_${version}_darwin_arm64.tar.gz";
        hash = "sha256-kH11CbkodKKWu9Nh3piGrdTAzSOV/o4Q24uzhasQUQU=";
      };
    };
    src = sources.${pkgs.stdenv.hostPlatform.system};
  in pkgs.stdenv.mkDerivation {
    pname = "gsemver";
    inherit version;
    src = pkgs.fetchurl { inherit (src) url hash; };
    sourceRoot = ".";
    dontConfigure = true;
    dontBuild = true;
    installPhase = ''
      install -Dm755 gsemver $out/bin/gsemver
    '';
  };

in
pkgs.mkShell {
  name = "go-devshell";

  buildInputs = with pkgs; [
    gnumake
    goPkg
    (golangci-lint.override { buildGo125Module = buildGo125Module.override { go = goPkg; }; })
    goreleaser
    gsemver
    kustomize
    kubernetes-helm
  ];

  shellHook = ''
    echo "go devshell (go.mod: ${fullVersion})"
    echo "GOPATH: $PWD/.go"
    export GOPATH=$PWD/.go
    mkdir -p $GOPATH
  '';
}
