#!/usr/bin/env bash

set -e

if [ ! -f docker-ovs-plugin.spec ]; then
  echo "No spec file" >&2
  exit 10
fi

declare -r RPMDIR=$(mktemp -d)
declare -r GITREF=$(git show-ref --hash HEAD)

_exit() {
  if [ -n "${RPMDIR}" ]; then
    rm -fr "${RPMDIR}"
  fi
}

trap "_exit" INT TERM QUIT EXIT

mkdir -p $(rpm -D "_topdir ${RPMDIR}" --eval "%{_rpmdir}")
mkdir -p $(rpm -D "_topdir ${RPMDIR}" --eval "%{_sourcedir}")
mkdir -p $(rpm -D "_topdir ${RPMDIR}" --eval "%{_specdir}")
mkdir -p $(rpm -D "_topdir ${RPMDIR}" --eval "%{_srcrpmdir}")
mkdir -p $(rpm -D "_topdir ${RPMDIR}" --eval "%{_builddir}")

cp docker-ovs-plugin.service $(rpm -D "_topdir ${RPMDIR}" --eval "%{_sourcedir}")/
# git submodule update --init --recursive
tar czpf $RPMDIR/SOURCES/${GITREF}.tar.gz --show-transformed --transform="s|./|./docker-ovs-plugin-${GITREF}/|" ./
rpmbuild -D "_topdir ${RPMDIR}" -ba docker-ovs-plugin.spec
cp -a $(rpm -D "_topdir ${RPMDIR}" --eval "%{_rpmdir}") $(rpm -D "_topdir ${RPMDIR}" --eval "%{_srcrpmdir}") ./

_exit
