#!/bin/bash -eu

source /build-common.sh

BINARY_NAME="pdfrasterizer"
COMPILE_IN_DIRECTORY="cmd/pdfrasterizer"

function maybeDownloadGhostscript {
	if [ -f gs ]; then
		return # already downloaded
	fi

	heading "Downloading Ghostscript"

	# thanks, champs..
	local downloadUrl="https://github.com/ArtifexSoftware/ghostpdl-downloads/releases/download/gs952/ghostscript-9.52-linux-x86_64.tgz"
	local directoryInTar="ghostscript-9.52-linux-x86_64"
	local binaryNameInTar="gs-952-linux-x86_64"

	# correct sha-256 is 3c235f005d31a0747617d3628b2313396ececda9669dbceba9ebda531b903578
	curl --fail --location "$downloadUrl" \
		| tar --strip-components=1 --wildcards -xzf - "*/$binaryNameInTar"

	mv "$binaryNameInTar" gs
}

# TODO: one deployerspec is done, we can stop overriding this from base image
function packageLambdaFunction {
	if [ ! -z ${FASTBUILD+x} ]; then return; fi

	cd rel/
	cp "${BINARY_NAME}_linux-amd64" "${BINARY_NAME}"
	rm -f lambdafunc.zip
	zip -j lambdafunc.zip "${BINARY_NAME}" "../gs"
	rm "${BINARY_NAME}"
}

maybeDownloadGhostscript

standardBuildProcess

packageLambdaFunction
