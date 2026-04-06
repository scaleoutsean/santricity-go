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
        api_endpoint => {
            description => "SANtricity controller IP address (e.g., 10.0.0.1).",
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
        api_username => {
            description => "API Username",
            type => 'string',
        },
        api_password => {
            description => "API Password",
            type => 'string',
        },
        insecure => {
            description => "Ignore TLS verify errors",
            type => 'boolean',
            default => 1,
        },
    };
}

sub options {
    return {
        vgname => { optional => 0 },
        base => { optional => 1 },
        saferemove => { optional => 1 },
        'snapshot-as-volume-chain' => { optional => 1 },
        api_endpoint => { optional => 0 },
        api_username => { optional => 0 },
        api_password => { optional => 0 },
        insecure => { optional => 1 },
        pool_id => { optional => 1 },
        host_group => { optional => 1 },
        host_name => { optional => 1 },
        shared => { optional => 0 },
        content => { optional => 1 },
        nodes => { optional => 1 },
        disable => { optional => 1 },
    };
}

# Ping wrapper function to shell out to our santricity-cli binary cleanly
sub ping_array {
    my ($storeid, $scfg) = @_;
    
    my $url = $scfg->{api_endpoint} || "";
    my $user = $scfg->{api_username} || "";
    
    # We allow standard config pass-through for testing, but prefer fetching from 
    # the secure /etc/pve/priv/ path using the storeid!
    my $pass = $scfg->{api_password} || "";
    if (!$pass && -e "/etc/pve/priv/storage/${storeid}.pw") {
        $pass = PVE::Tools::file_read_firstline("/etc/pve/priv/storage/${storeid}.pw") // "";
    }
    
    return 0 if (!$url || !$user || !$pass);
    
    # Check if insecure flag is explicitly defined and disabled, otherwise default to inserting it
    my $insecure_flag = "--insecure";
    if (defined($scfg->{insecure}) && $scfg->{insecure} == 0) {
        $insecure_flag = "";
    }
    
    # We use system() here instead of PVE's run_command because run_command
    # can throw fatal dies in strict daemon contexts, and we also want to 
    # aggressively black-hole the STDOUT/STDERR from Go's log.Printf cleanly.
    my $cmd = "/usr/local/bin/santricity-cli --endpoint '$url' --username '$user' --password '$pass' $insecure_flag get system >/dev/null 2>&1";
    system($cmd);
    
    # system() exit code 0 means success.
    return ($? == 0);
}

# Override `status` to do a quick ping to the array via the CLI.
# In Proxmox, `pvestatd` runs this locally on every node, giving us per-node reachability status!
sub status {
    my ($class, $storeid, $scfg, $cache) = @_;

    # Proxmox storage status expects an array map: ($total, $free, $used, $active)
    # First, get the underlying LVM VG footprint natively
    my @lvm_status = $class->SUPER::status($storeid, $scfg, $cache);
    
    # If LVM is inactive locally, immediately return its offline state
    return @lvm_status if !@lvm_status || !$lvm_status[3]; 
    
    # Perform a sanity check via CLI to verify the array management API is reachable.
    # If the array is down from this specific node, gracefully map it as inactive.
    if (!ping_array($storeid, $scfg)) {
        $lvm_status[3] = 0;
    }
    
    return @lvm_status;
}

1;
