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

in
pkgs.mkShell {
  name = "go-devshell";

  buildInputs = with pkgs; [
    goPkg
    golangci-lint
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
