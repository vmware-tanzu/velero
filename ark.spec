#debuginfo not supported with Go
%global debug_package %{nil}

%global golang_version 1.6.2

%global gopath      %{_datadir}/gocode
%define ark_deps %{gopath}/src/github.com/heptio/ark

Name:           ark
Version:        0.3.3
Release:        1%{?dist}
Summary:        Ark is a utility for managing disaster recovery.
License:        ASL 2.0
URL:            https://github.com/heptio/ark

# If go_arches not defined fall through to implicit golang archs
%if 0%{?go_arches:1}
ExclusiveArch:  %{go_arches}
%else
ExclusiveArch:  x86_64 aarch64 ppc64le s390x
%endif

Source0:        %{name}-%{version}.tar.xz
BuildRequires:  golang >= %{golang_version}

%description
Heptio Ark is a utility for managing disaster recovery, specifically for your Kubernetes cluster resources and persistent volumes. Brought to you by Heptio.

%prep
%setup -q

%build
export GOPATH=%{gopath}
if [ ! -d %{ark_deps} ]; then
    mkdir -p %{ark_deps}
    /usr/bin/cp -ar ./* %{ark_deps}/
fi
make

%install
PLATFORM="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
install -d %{buildroot}%{_bindir}
install -p -m 755 _output/bin/ark %{buildroot}%{_bindir}/ark

%files
%{_bindir}/ark

%pre

%changelog
* Sat Aug 12 2017 Vanlos Wang vanloswang@126.com
- Initial Package.
