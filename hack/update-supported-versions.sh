#!/usr/bin/env bash

echo "| Argo CD version | Kubernetes versions |" > docs/operator-manual/tested-kubernetes-versions.md
echo "|-----------------|---------------------|" >> docs/operator-manual/tested-kubernetes-versions.md

argocd_minor_version=$(git rev-parse --abbrev-ref HEAD | sed 's/release-//')
argocd_major_version_num=$(echo "$argocd_minor_version" | sed -E 's/\.[0-9]+//')
argocd_minor_version_num=$(echo "$argocd_minor_version" | sed -E 's/[0-9]+\.//')

for n in 0 1 2; do
  minor_version_num=$((argocd_minor_version_num - n))
  minor_version="${argocd_major_version_num}.${minor_version_num}"
  echo 0
  if [ $n -ne 0 ]; then git stash pop; fi
  echo 1
  git checkout "release-$minor_version" > /dev/null || exit 1
  echo 2
  yq '.jobs["test-e2e"].strategy.matrix["k3s-version"][]' .github/workflows/ci-build.yaml | \
    jq --arg minor_version "$minor_version" --raw-input --slurp --raw-output \
    'split("\n")[:-1] | map(sub("\\.[0-9]+$"; "")) | join(", ") | "| \($minor_version) | \(.) |"' \
    >> docs/operator-manual/tested-kubernetes-versions.md
  echo 3
  git add docs/operator-manual/tested-kubernetes-versions.md
  echo 4
  git stash
done

git checkout "release-$argocd_minor_version"

git stash pop

echo >> docs/operator-manual/tested-kubernetes-versions.md
