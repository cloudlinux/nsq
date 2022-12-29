%define debug_package %{nil}

Name: imunify-nsq
Version: 1.0.1
Release: 1%{?dist}
Summary: Imunify NSQ
License: CLOUD LINUX LICENSE AGREEMENT
URL: https://github.com/cloudlinux/nsq
Source0: %{name}-%{version}.tar.gz
ExcludeArch: %{ix86}
BuildRequires: autoconf
BuildRequires: automake
BuildRequires: libtool
BuildRequires: make
BuildRequires: unzip
%if %{rhel} > 6
%if %{rhel} > 7
BuildRequires: systemd-rpm-macros
%{?systemd_requires}
%else
BuildRequires: systemd
%endif
%else
Requires: util-linux-ng
%endif

%description
NSQ message queue with unix socket listener support.
This service receives and routes messages across services.

%prep
%setup -q

%build
make

%install
make install DESTDIR=%{buildroot}
%if %{rhel} > 6
    mkdir -p %{buildroot}/%{_unitdir}
    install -m 0644 systemd/%{name}.service %{buildroot}/%{_unitdir}
%else
    mkdir -p %{buildroot}/%{_initddir}
    install -m 0755 init.d/%{name}.init.d %{buildroot}/%{_initddir}/%{name}
%endif

%{__mkdir} -p $RPM_BUILD_ROOT%{_sysconfdir}/logrotate.d
install -m 644 imunify-nsq.logrotate $RPM_BUILD_ROOT%{_sysconfdir}/logrotate.d/imunify-nsq

%check

%files
%{_sbindir}/imunify360-nsqd
%dir /var/lib/imunify-nsqd
%config %{_sysconfdir}/logrotate.d/imunify-nsq

%if %{rhel} > 6
%license LICENSE
%{_unitdir}/%{name}.service
%else
# license macro is not working on CentOS 6
%doc LICENSE
%{_initddir}/%{name}
%endif

%post
%if %{rhel} > 6
%systemd_post %{name}.service
%else
if [ $1 -eq 1 ] ; then
    # Initial installation
    chkconfig --add %{name} || :
fi
%endif

%preun
%if %{rhel} > 6
%systemd_preun %{name}.service
%else
if [ $1 -eq 0 ] ; then
    # Package removal, not upgrade
    service %{name} stop || :
    chkconfig --del %{name} || :
fi
%endif

%postun
%if %{rhel} > 6
%systemd_postun_with_restart %{name}.service
%else
if [ $1 -ge 1 ] ; then
    # Package upgrade, not uninstall
    service try-restart %{name}
fi
%endif

if [ $1 -eq 0 ] ; then
    # uninstall
    rm -rf /var/lib/imunify-nsqd
fi

%changelog
* Mon Dec 26 2022 Mikhail Faraponov <mfaraponov@cloudlinux.com> - 1.0.1-1
- Initial release
