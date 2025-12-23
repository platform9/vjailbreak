# Cinder POC

A proof-of-concept HTTP server demonstrating OpenStack Cinder volume operations for VJailbreak migration.

## Structure

```
cinder-poc/
├── cmd/demo/           # HTTP server entry point
│   └── main.go
├── pkg/
│   ├── cinder/         # Cinder storage provider implementation
│   │   └── provider.go
│   └── storage/        # Storage types and interfaces
│       └── types.go
├── go.mod              # Go module definition
├── Makefile            # Build and test commands
└── openstack.rc        # OpenStack credentials (source this)
```

## Prerequisites

- Go 1.21+
- Access to OpenStack Cinder (credentials in `openstack.rc`)
- `curl` and `jq` for testing

## Quick Start

### 1. Install dependencies

```bash
cd /Users/sanya-pf9/Developer/vjailbreak/cinder-poc
go mod download
```

### 2. Start the server

```bash
make run
```

This will:
- Source `openstack.rc` to load credentials
- Start the HTTP server on port 8080
- Log output to `server.log`

### 3. Test the connection

```bash
make test-connect
```

### 4. List volumes

```bash
make test-list
```

## Available Commands

Run `make help` to see all available commands:

```bash
make help
```

### Server Management
- `make run` - Start server in background
- `make stop` - Stop the server
- `make restart` - Restart the server
- `make logs` - Tail server logs
- `make clean` - Clean up logs

### Testing Endpoints
- `make test-connect` - Test OpenStack connection
- `make test-whoami` - Check provider type
- `make test-list` - List all volumes
- `make test-create` - Create a test volume
- `make test-get` - Get volume info
- `make test-map` - Map volume to host (requires IQN)
- `make test-unmap` - Unmap volume from host

### Example: Create and Map a Volume

```bash
# Create a volume
make test-create VOLUME_NAME=my-vol SIZE_BYTES=2147483648

# Map it to a host (replace with your IQN)
make test-map VOLUME_NAME=my-vol HOST=esx-01 IQN=iqn.1998-01.com.vmware:your-host
```

## API Endpoints

### POST /connect
Connect to OpenStack and validate credentials.

### POST /create-volume
Create a new Cinder volume.

**Body:**
```json
{
  "name": "vol1",
  "sizeBytes": 1073741824,
  "volumeType": "pure-sc"
}
```

### POST /map-volume
Map a volume to host initiators.

**Body:**
```json
{
  "initiatorGroupName": "esx-group",
  "volumeName": "vol1",
  "iqns": ["iqn.1998-01.com.vmware:hostname"]
}
```

### POST /unmap-volume
Unmap a volume from host initiators.

**Body:**
```json
{
  "initiatorGroupName": "esx-group",
  "volumeName": "vol1",
  "iqns": ["iqn.1998-01.com.vmware:hostname"]
}
```

### GET /get-volume?name=vol1
Get information about a specific volume.

### GET /list-volumes
List all volumes.

### GET /whoami
Returns the provider type ("cinder").

## Configuration

Edit `openstack.rc` to configure your OpenStack credentials:

```bash
export OS_USERNAME=your-username
export OS_PASSWORD=your-password
export OS_AUTH_URL=https://your-keystone:5000/v3
export OS_PROJECT_NAME=your-project
export OS_REGION_NAME=RegionOne
```

## Development

```bash
# Format code
make fmt

# Tidy dependencies
make tidy

# Build binary
make build
```

## Troubleshooting

### Connection timeout
- Check that `openstack.rc` has correct credentials
- Verify network connectivity to OpenStack endpoint
- Check if you need VPN access

### Server won't start
- Check if port 8080 is already in use: `lsof -i :8080`
- View logs: `make logs-all`

### Volume operations fail
- Ensure you're connected: `make test-connect`
- Check Cinder service status in OpenStack
- Verify you have necessary permissions
