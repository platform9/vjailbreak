#!/bin/bash

if [ -f /home/fedora/SYSTEM ]; then
    if [ ! -d /home/fedora/registry ]; then
        mkdir /home/fedora/registry
    fi
    reged -x /home/fedora/SYSTEM "HKEY_LOCAL_MACHINE\SYSTEM" "\ControlSet001\Control\Network" /home/fedora/registry/network.reg
    reged -x /home/fedora/SYSTEM "HKEY_LOCAL_MACHINE\SYSTEM" "\ControlSet001\Control\Class\{4d36e972-e325-11ce-bfc1-08002be10318}" /home/fedora/registry/class.reg
    reged -x /home/fedora/SYSTEM "HKEY_LOCAL_MACHINE\SYSTEM" "\ControlSet001\Services" /home/fedora/registry/service.reg


fi