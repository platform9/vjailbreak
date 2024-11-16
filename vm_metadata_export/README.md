## Usage

```
vmmetadataexporter -username=<name> -password=<password> -host=<vcenter.phx.pnap.platform9.horse>
```

## Example

```
$ vmmetadataexporter -username=topsecretuser -password=topsecretpassword -host=vcenter.somelocation.com
Connected to vCenter
Retrieved 139 VMs
Processing VM 0
Processing VM 10
Processing VM 20
Processing VM 30
Processing VM 40
Processing VM 50
Processing VM 60
Processing VM 70
Processing VM 80
Processing VM 90
Processing VM 100
Processing VM 110
Processing VM 120
Processing VM 130
Converted to CSV
```

## Output

Generates a csv file called vms.csv in the same directory as binary containing the details

    Name
    OS Details
    Disk Size (Bytes)
    RDM
    Independent Disks
    VTPM
    Encrypted

## Build

```
go mod init
go mod tidy

# For Linux
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o vmmetadataexporter main.go

# For Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -o vmmetadataexporter.exe main.go
```