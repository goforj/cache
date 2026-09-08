#!/usr/bin/env bash
set -euo pipefail

sbom_directory="${1:?SBOM directory is required}"
output="${2:?output file is required}"
sha="${3:?commit SHA is required}"
ref="${4:?Git ref is required}"
job_id="${5:?job ID is required}"
job_url="${6:?job URL is required}"
scanned="${7:?scan time is required}"

mapfile -t sboms < <(find "${sbom_directory}" -maxdepth 1 -type f -name '*.cdx.json' | sort)
[[ "${#sboms[@]}" -gt 0 ]]

jq -s \
  --arg sha "${sha}" \
  --arg ref "${ref}" \
  --arg job_id "${job_id}" \
  --arg job_url "${job_url}" \
  --arg scanned "${scanned}" \
  '
    def clean_purl: split("?")[0];
    def manifest:
      . as $bom |
      ([
        $bom.metadata.properties[] |
        select(.name == "goforj:manifest-source") |
        .value
      ][0]) as $source |
      ($bom.metadata.component["bom-ref"]) as $root |
      ([
        $bom.dependencies[]? |
        select(.ref == $root) |
        .dependsOn[]?
      ]) as $direct |
      (($bom.components // []) |
        map(select(.purl != null) | {
          key: .["bom-ref"],
          value: (.purl | clean_purl)
        }) |
        from_entries
      ) as $references |
      {
        key: $source,
        value: {
          name: $source,
          file: {source_location: $source},
          resolved: (reduce (($bom.components // [])[] | select(.purl != null)) as $component ({};
            ($component["bom-ref"]) as $component_ref |
            .[($component.purl | clean_purl)] = {
              package_url: ($component.purl | clean_purl),
              relationship: (if ($direct | index($component_ref)) then "direct" else "indirect" end),
              scope: (if $component.scope == "optional" then "development" else "runtime" end),
              dependencies: ([
                $bom.dependencies[]? |
                select(.ref == $component_ref) |
                .dependsOn[]? |
                $references[.]
              ] | map(select(. != null)) | unique)
            }
          ))
        }
      };
    {
      version: 0,
      sha: $sha,
      ref: $ref,
      job: {
        correlator: "goforj-resolved-go-modules-v1",
        id: $job_id,
        html_url: $job_url
      },
      detector: {
        name: "goforj/cyclonedx-gomod",
        version: "1.12.0",
        url: "https://github.com/CycloneDX/cyclonedx-gomod"
      },
      scanned: $scanned,
      manifests: (map(manifest) | from_entries)
    }
  ' "${sboms[@]}" > "${output}"

jq -e \
  --arg sha "${sha}" \
  --arg ref "${ref}" \
  --argjson expected_manifests "${#sboms[@]}" \
  '
    .sha == $sha and
    .ref == $ref and
    (.manifests | length) == $expected_manifests and
    ([.manifests | keys[] | select(length == 0)] | length) == 0 and
    ([.manifests[].resolved[]? | select(
      (.package_url | startswith("pkg:golang/")) == false or
      (.relationship != "direct" and .relationship != "indirect") or
      (.scope != "runtime" and .scope != "development")
    )] | length) == 0
  ' "${output}" > /dev/null

printf 'Dependency snapshot manifests: %s\n' "${#sboms[@]}"
