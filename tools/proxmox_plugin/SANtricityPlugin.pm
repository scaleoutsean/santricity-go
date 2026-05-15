package PVE::Storage::Custom::SANtricityPlugin;

use strict;
use warnings;

use PVE::Storage::Plugin;
use PVE::Storage::LVMPlugin;
use PVE::Tools qw(run_command);
use PVE::Cluster qw(cfs_read_file cfs_write_file cfs_lock_file);

# Inherit from Proxmox's native LVM Plugin (The Big LUN architecture)
use base qw(PVE::Storage::LVMPlugin);

# Plugin Identifier (Type)
sub type {
    return 'santricity_lvm';
}

sub api {
    # Dynamically match the Proxmox Plugin API version (e.g., v10 for PVE 8, v13 for PVE 9)
    # This prevents the "does not provide an api() method" error.
    return eval { PVE::Storage::APIVER } || 10;
}

sub plugindata {
    return {
        content => [ {images => 1, rootdir => 1}, { images => 1 }],
        format => [ { raw => 1 } , 'raw' ],
    };
}

# Add our custom SANtricity Array connection parameters
# Not all are currently required, but they may be used in status() to determine array reachability and map it to the storage status in Proxmox.
sub properties {
    return {
        array_serial => {
            description => "The 12-digit Chassis Serial Number of the SANtricity array.",
            type => 'string',
        },
        pool_id => {
            description => "The target Dynamic Disk Pool (DDP) ID on the E-Series array.",
            type => 'string',
        },
        host_group => {
            description => "The Host Group ID that represents the Proxmox cluster locally on the array.",
            type => 'string',
        },
        host_name => {
            description => "The individual Host ID (for single-node PVE or DAS setups).",
            type => 'string',
        },
    };
}

sub options {
    return {
        vgname => { optional => 0 },
        base => { optional => 1 },
        saferemove => { optional => 1 },
        'snapshot-as-volume-chain' => { optional => 1 },
        array_serial => { optional => 0 },
        pool_id => { optional => 1 },
        host_group => { optional => 1 },
        host_name => { optional => 1 },
        shared => { optional => 0 },
        content => { optional => 1 },
        nodes => { optional => 1 },
        disable => { optional => 1 },
    };
}

1;
