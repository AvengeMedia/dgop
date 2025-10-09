# Modular spec for dgop - stable and git builds
#
# Build types controlled by %git_build macro:
# - git_build=1 (default): Build from latest git commit (dgop-git package)
# - git_build=0: Build from tagged release (dgop package)

%global debug_package %{nil}

# Set build type - override with --define 'git_build 0' for stable releases
%{!?git_build: %global git_build 1}

%if %{git_build}
# Git build - use rpkg git macros
%global version {{{ git_dir_version }}}
%global pkg_summary System monitoring CLI and REST API (git development version)
%else
# Stable build - use tagged version
%global version 0.1.4
%global pkg_summary System monitoring CLI and REST API
%endif

Name:           dgop
Version:        %{version}
Release:        1%{?dist}
Summary:        %{pkg_summary}

License:        MIT
URL:            https://github.com/AvengeMedia/dgop

%if %{git_build}
VCS:            {{{ git_dir_vcs }}}
Source0:        {{{ git_dir_pack }}}
%else
Source0:        https://github.com/AvengeMedia/dgop/archive/refs/tags/v%{version}.tar.gz#/dgop-%{version}.tar.gz
%endif

BuildRequires:  git-core
BuildRequires:  golang >= 1.21
BuildRequires:  rpkg

Requires:       glibc

%description
dgop is a go-based stateless system monitoring tool that provides both a CLI interface
and REST API for retrieving system metrics including CPU, memory, disk, network,
processes, and GPU information.

%if %{git_build}
This is the development version built from the latest git commit.
%endif

Features:
- Interactive TUI with real-time system monitoring
- REST API server with OpenAPI specification
- JSON output for all metrics
- GPU temperature monitoring (NVIDIA)
- Lightweight single-binary deployment

%prep
%if %{git_build}
{{{ git_dir_setup_macro }}}
%else
%autosetup -n dgop-%{version}
%endif

%build
export CGO_CPPFLAGS="${CPPFLAGS}"
export CGO_CFLAGS="${CFLAGS}"
export CGO_CXXFLAGS="${CXXFLAGS}"
export CGO_LDFLAGS="${LDFLAGS}"
export GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"

# Build the binary
go build -v -o dgop ./cmd/cli

%install
install -Dm755 dgop %{buildroot}%{_bindir}/dgop

%files
%license LICENSE
%doc README.md
%{_bindir}/dgop

%changelog
%if %{git_build}
{{{ git_dir_changelog }}}
%else
* Thu Oct 09 2025 AvengeMedia <support@avengemedia.net> - 0.1.4-1
- Update to v0.1.4 stable release
%endif
