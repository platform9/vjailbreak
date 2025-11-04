# VM Migration Architecture Diagrams

## 1. System Context Diagram

```mermaid
graph TB
    subgraph "VMware Environment"
        vCenter[vCenter Server]
        ESXi1[ESXi Host 1]
        ESXi2[ESXi Host 2]
        ESXiN[ESXi Host N]
        VMFS[VMFS Datastores]
    end
    
    subgraph "Migration Controller"
        API[Migration API]
        Controller[Migration Controller]
        StorageMapper[Storage Mapper]
        TaskManager[Task Manager]
        SSHClient[SSH Client]
    end
    
    subgraph "Storage Arrays"
        Pure[Pure Storage]
        NetApp[NetApp ONTAP]
        Other[Other Arrays]
    end
    
    subgraph "OpenStack Environment"
        Nova[Nova]
        Cinder[Cinder]
        PCD[Platform9 PCD]
        TargetVol[Target Volumes]
    end
    
    User[User/Admin] --> API
    API --> Controller
    Controller --> StorageMapper
    Controller --> TaskManager
    Controller --> SSHClient
    
    vCenter -.Query VMs.-> Controller
    SSHClient -.SSH Commands.-> ESXi1
    SSHClient -.SSH Commands.-> ESXi2
    SSHClient -.SSH Commands.-> ESXiN
    
    ESXi1 --> VMFS
    ESXi2 --> VMFS
    ESXiN --> VMFS
    
    StorageMapper -.Storage API.-> Pure
    StorageMapper -.Storage API.-> NetApp
    StorageMapper -.Storage API.-> Other
    
    VMFS -.Backed by.-> Pure
    VMFS -.Backed by.-> NetApp
    VMFS -.Backed by.-> Other
    
    Controller --> Cinder
    Cinder --> TargetVol
    
    ESXi1 -.XCOPY.-> TargetVol
    ESXi2 -.XCOPY.-> TargetVol
    ESXiN -.XCOPY.-> TargetVol
    
    style Controller fill:#e1f5ff
    style StorageMapper fill:#fff4e1
    style Pure fill:#ffe1e1
    style NetApp fill:#ffe1e1
    style Other fill:#ffe1e1
```

## 2. Complete Clone Operation Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant API
    participant Controller
    participant StorageMapper
    participant vCenter
    participant Cinder
    participant StorageArray
    participant ESXi
    participant TargetVolume
    
    User->>API: POST /migrate (VM details)
    API->>Controller: Start Migration
    
    Note over Controller: Phase 1: Discovery & Validation
    Controller->>vCenter: Get VM disk details
    vCenter-->>Controller: disk paths, datastore IDs
    
    Controller->>Controller: Lookup datastore→array mapping<br/>(from StorageArrayMapping CR)
    
    Controller->>Cinder: Create target volume<br/>(matching volume type)
    Cinder-->>Controller: Volume ID, NAA/WWN
    
    Note over Controller: Phase 2: Storage Preparation
    Controller->>StorageMapper: GetStorageOperator(vendor)
    StorageMapper-->>Controller: Pure/NetApp/etc operator
    
    Controller->>ESXi: Get HBA identifiers (SSH)
    ESXi-->>Controller: IQNs/WWPNs
    
    Controller->>StorageArray: CreateOrUpdateInitiatorGroup<br/>("xcopy-esxi", [IQNs])
    StorageArray-->>Controller: MappingContext
    
    Controller->>StorageArray: GetMappedGroups(targetVolume)
    StorageArray-->>Controller: Current mappings
    
    Controller->>StorageArray: MapVolumeToGroup<br/>("xcopy-esxi", targetVolume)
    StorageArray-->>Controller: Mapping complete
    
    Note over ESXi: Volume now visible to ESXi
    
    Controller->>ESXi: Rescan storage adapters (SSH)
    ESXi-->>Controller: Rescan complete
    
    Note over Controller: Phase 3: Clone Execution
    Controller->>ESXi: Execute async clone<br/>vmkfstools clone<br/>-s [ds]/vm.vmdk<br/>-t /dev/disk/naa.xxx
    ESXi-->>Controller: Task ID: abc-123-def
    
    loop Progress Monitoring
        Controller->>ESXi: vmkfstools taskGet -i abc-123
        ESXi-->>Controller: Progress: 45%
        Controller->>User: Update progress
    end
    
    ESXi-->>Controller: Clone complete (100%)
    
    Note over Controller: Phase 4: Cleanup
    Controller->>ESXi: vmkfstools taskClean -i abc-123
    ESXi-->>Controller: Cleanup complete
    
    Controller->>StorageArray: UnmapVolumeFromGroup<br/>("xcopy-esxi", targetVolume)
    StorageArray-->>Controller: Unmapped
    
    Controller->>StorageArray: GetMappedGroups(targetVolume)
    StorageArray-->>Controller: Verify original state
    
    Controller->>ESXi: Rescan storage adapters
    ESXi-->>Controller: Rescan complete
    
    Controller->>API: Migration complete
    API->>User: Success with volume details
```

## 3. Storage Interface Architecture

```mermaid
graph TB
    subgraph "Migration Controller"
        Controller[Migration Controller]
        Factory[Storage Provider Factory]
    end
    
    subgraph "Storage Abstraction Layer"
        Interface[StorageOperator Interface]
        
        subgraph "Interface Methods"
            Mapper[StorageMapper]
            Resolver[VolumeResolver]
        end
        
        Interface --> Mapper
        Interface --> Resolver
    end
    
    subgraph "Storage Provider Implementations"
        PureImpl[Pure Storage Provider]
        NetAppImpl[NetApp Provider]
        HitachiImpl[Hitachi Provider]
        GenericImpl[Generic Provider]
    end
    
    subgraph "Storage APIs"
        PureAPI[Pure REST API]
        NetAppAPI[ONTAP REST API]
        HitachiAPI[Hitachi API]
    end
    
    Controller --> Factory
    Factory -.selects based on vendor.-> PureImpl
    Factory -.selects based on vendor.-> NetAppImpl
    Factory -.selects based on vendor.-> HitachiImpl
    Factory -.selects based on vendor.-> GenericImpl
    
    PureImpl -.implements.-> Interface
    NetAppImpl -.implements.-> Interface
    HitachiImpl -.implements.-> Interface
    GenericImpl -.implements.-> Interface
    
    PureImpl --> PureAPI
    NetAppImpl --> NetAppAPI
    HitachiImpl --> HitachiAPI
    
    style Interface fill:#e1f5ff
    style Mapper fill:#fff4e1
    style Resolver fill:#fff4e1
```

## 4. StorageMapper Interface Methods Flow

```mermaid
graph LR
    subgraph "CreateOrUpdateInitiatorGroup"
        A1[Input: groupName, HBA IDs]
        A2[Create/Update igroup on array]
        A3[Add IQNs/WWPNs to group]
        A4[Return: MappingContext]
        A1 --> A2 --> A3 --> A4
    end
    
    subgraph "MapVolumeToGroup"
        B1[Input: groupName, Volume]
        B2[Create LUN mapping]
        B3[Volume visible to ESXi]
        B4[Return: Updated Volume]
        B1 --> B2 --> B3 --> B4
    end
    
    subgraph "GetMappedGroups"
        C1[Input: Volume]
        C2[Query array for mappings]
        C3[Return: List of groups]
        C1 --> C2 --> C3
    end
    
    subgraph "UnmapVolumeFromGroup"
        D1[Input: groupName, Volume]
        D2[Remove LUN mapping]
        D3[Volume hidden from ESXi]
        D4[Return: Success/Error]
        D1 --> D2 --> D3 --> D4
    end
    
    style A2 fill:#ffe1e1
    style B2 fill:#ffe1e1
    style C2 fill:#ffe1e1
    style D2 fill:#ffe1e1
```

## 5. Task State Machine

```mermaid
stateDiagram-v2
    [*] --> Pending: Create migration request
    
    Pending --> Validating: Start migration
    Validating --> Failed: Validation error
    Validating --> Preparing: Validation passed
    
    Preparing --> Failed: Storage prep error
    Preparing --> Mapping: Target volume ready
    
    Mapping --> Failed: Mapping error
    Mapping --> Cloning: Volume mapped to ESXi
    
    Cloning --> Cloning: Progress update (0-100%)
    Cloning --> Failed: Clone error
    Cloning --> Unmapping: Clone complete
    
    Unmapping --> Warning: Unmap partial failure
    Unmapping --> Completed: Cleanup successful
    
    Warning --> Completed: Manual cleanup
    
    Failed --> [*]: Cleanup & report
    Completed --> [*]: Success
    
    note right of Validating
        - Check VM exists
        - Check disk space
        - Verify connectivity
    end note
    
    note right of Mapping
        - Create initiator group
        - Map target volume
        - Rescan ESXi adapters
    end note
    
    note right of Cloning
        - Execute vmkfstools
        - Poll progress
        - Monitor for errors
    end note
    
    note right of Unmapping
        - Clean task artifacts
        - Unmap volume
        - Restore original state
    end note
```

## 6. SSH vs VIB Architecture Comparison

```mermaid
graph TB
    subgraph "SSH-Based Architecture"
        SSH_Controller[Migration Controller]
        SSH_Client[SSH Client with Keys]
        SSH_Script[Python Wrapper Script<br/>on Datastore]
        SSH_Restricted[Restricted SSH Key<br/>command= limitation]
        SSH_VMK[vmkfstools Process]
        
        SSH_Controller --> SSH_Client
        SSH_Client --> SSH_Restricted
        SSH_Restricted --> SSH_Script
        SSH_Script --> SSH_VMK
    end
    
    subgraph "VIB-Based Architecture"
        VIB_Controller[Migration Controller]
        VIB_vCenter[vCenter API]
        VIB_ESXCLI[esxcli Framework]
        VIB_Plugin[Custom VIB Plugin<br/>/usr/lib/vmware/esxcli/]
        VIB_Wrapper[Wrapper Script<br/>/opt/pf9/]
        VIB_VMK[vmkfstools Process]
        
        VIB_Controller --> VIB_vCenter
        VIB_vCenter --> VIB_ESXCLI
        VIB_ESXCLI --> VIB_Plugin
        VIB_Plugin --> VIB_Wrapper
        VIB_Wrapper --> VIB_VMK
    end
    
    SSH_Pros["✓ No installation required<br/>✓ No reboot needed<br/>✓ Easy updates<br/>✓ Better for security-restricted"]
    
    VIB_Pros["✓ vCenter API access<br/>✓ Native ESXi integration<br/>✓ Persistent across reboots<br/>✓ Standard VMware method"]
    
    style SSH_Controller fill:#e1ffe1
    style VIB_Controller fill:#e1e1ff
    style SSH_Pros fill:#e1ffe1
    style VIB_Pros fill:#e1e1ff
```

## 7. Datastore to Storage Array Mapping

```mermaid
graph TB
    subgraph "vCenter Environment"
        DS1[Datastore: ds-1234<br/>Name: production-ssd]
        DS2[Datastore: ds-5678<br/>Name: dev-nvme]
        DS3[Datastore: ds-9012<br/>Name: backup-sata]
    end
    
    subgraph "StorageArrayMapping CR"
        CR[Custom Resource]
        Map1[Mapping 1:<br/>datastoreId: ds-1234<br/>vendor: pure<br/>volumeType: flash-tier1]
        Map2[Mapping 2:<br/>datastoreId: ds-5678<br/>vendor: ontap<br/>volumeType: ssd-tier2]
        Map3[Mapping 3:<br/>datastoreId: ds-9012<br/>vendor: hitachi<br/>volumeType: sata-backup]
        
        CR --> Map1
        CR --> Map2
        CR --> Map3
    end
    
    subgraph "Physical Storage Arrays"
        Pure[Pure FlashArray<br/>192.168.1.100]
        NetApp[NetApp ONTAP<br/>192.168.1.200]
        Hitachi[Hitachi VSP<br/>192.168.1.300]
    end
    
    subgraph "OpenStack Cinder"
        Vol1[Volume Type: flash-tier1<br/>Backend: Pure]
        Vol2[Volume Type: ssd-tier2<br/>Backend: NetApp]
        Vol3[Volume Type: sata-backup<br/>Backend: Hitachi]
    end
    
    DS1 -.backed by.-> Pure
    DS2 -.backed by.-> NetApp
    DS3 -.backed by.-> Hitachi
    
    Map1 -.maps to.-> Vol1
    Map2 -.maps to.-> Vol2
    Map3 -.maps to.-> Vol3
    
    Vol1 --> Pure
    Vol2 --> NetApp
    Vol3 --> Hitachi
    
    Note1[Goal: Create Cinder volume<br/>on SAME array as<br/>source VMDK datastore]
    
    style CR fill:#fff4e1
    style Note1 fill:#e1f5ff
```

## 8. Component Architecture Detail

```mermaid
graph TB
    subgraph "API Layer"
        REST[REST API Server]
        Auth[Authentication]
        Validation[Request Validation]
    end
    
    subgraph "Core Controllers"
        MigCtrl[Migration Controller]
        TaskCtrl[Task Controller]
        StateCtrl[State Manager]
    end
    
    subgraph "Storage Abstraction"
        StorageFactory[Provider Factory]
        StorageMapper[Storage Mapper]
        VolumeResolver[Volume Resolver]
    end
    
    subgraph "ESXi Integration"
        SSHMgr[SSH Manager]
        KeyMgr[Key Manager]
        CmdExec[Command Executor]
        VIBMgr[VIB Manager]
    end
    
    subgraph "External Integrations"
        vCenterClient[vCenter Client]
        CinderClient[Cinder Client]
        K8sClient[Kubernetes Client]
    end
    
    subgraph "Data Layer"
        CRDs[Custom Resources<br/>StorageArrayMapping<br/>MigrationTask]
        Secrets[Kubernetes Secrets<br/>SSH Keys<br/>Storage Credentials]
        ConfigMaps[Configuration]
    end
    
    REST --> Auth
    Auth --> Validation
    Validation --> MigCtrl
    
    MigCtrl --> TaskCtrl
    MigCtrl --> StateCtrl
    MigCtrl --> StorageFactory
    MigCtrl --> SSHMgr
    MigCtrl --> vCenterClient
    MigCtrl --> CinderClient
    
    StorageFactory --> StorageMapper
    StorageFactory --> VolumeResolver
    
    SSHMgr --> KeyMgr
    SSHMgr --> CmdExec
    
    TaskCtrl --> K8sClient
    K8sClient --> CRDs
    
    KeyMgr --> Secrets
    MigCtrl --> ConfigMaps
    
    style MigCtrl fill:#e1f5ff
    style StorageFactory fill:#fff4e1
    style SSHMgr fill:#ffe1e1
```

## 9. Error Recovery Flow

```mermaid
graph TB
    Start[Migration Started]
    
    Start --> Validate{Validation}
    Validate -->|Success| PrepStorage
    Validate -->|Fail| ErrValidate[Log Error & Return]
    
    PrepStorage[Create Target Volume]
    PrepStorage --> ChkVol{Volume<br/>Created?}
    ChkVol -->|Yes| MapStorage
    ChkVol -->|No| ErrVol[Cleanup & Return]
    
    MapStorage[Map Volume to ESXi]
    MapStorage --> ChkMap{Mapping<br/>Success?}
    ChkMap -->|Yes| Clone
    ChkMap -->|No| ErrMap[Delete Volume<br/>& Return]
    
    Clone[Execute Clone]
    Clone --> ChkClone{Clone<br/>Success?}
    ChkClone -->|Yes| Unmap
    ChkClone -->|No| ErrClone[Unmap Volume<br/>Delete Volume<br/>& Return]
    
    Unmap[Unmap Volume]
    Unmap --> ChkUnmap{Unmap<br/>Success?}
    ChkUnmap -->|Yes| Success[Complete]
    ChkUnmap -->|No| Warning[Complete with<br/>Warning<br/>Manual cleanup needed]
    
    ErrValidate --> End[End]
    ErrVol --> End
    ErrMap --> End
    ErrClone --> End
    Success --> End
    Warning --> End
    
    style Success fill:#e1ffe1
    style Warning fill:#fffee1
    style ErrValidate fill:#ffe1e1
    style ErrVol fill:#ffe1e1
    style ErrMap fill:#ffe1e1
    style ErrClone fill:#ffe1e1
```

## 10. SSH Key Setup and Security Flow

```mermaid
sequenceDiagram
    participant Admin
    participant System
    participant K8sSecrets
    participant ESXi
    participant Datastore
    
    Note over Admin,ESXi: Option 1: System-Generated Keys
    
    Admin->>System: Deploy migration controller
    System->>System: Generate 2048-bit RSA keypair
    System->>K8sSecrets: Store private key
    System->>K8sSecrets: Store public key
    System->>Admin: Display public key
    
    Admin->>Admin: Copy public key
    Admin->>ESXi: Enable SSH service
    Admin->>Datastore: Upload secure-vmkfstools-wrapper.py
    Admin->>ESXi: Create restricted authorized_keys entry<br/>command="python /vmfs/volumes/.../wrapper.py"
    
    Note over Admin,ESXi: Option 2: User-Provided Keys
    
    Admin->>Admin: Generate own keypair<br/>ssh-keygen -t rsa -b 4096
    Admin->>K8sSecrets: Upload private key
    Admin->>K8sSecrets: Upload public key
    Admin->>ESXi: Enable SSH service
    Admin->>Datastore: Upload secure-vmkfstools-wrapper.py
    Admin->>ESXi: Create restricted authorized_keys entry
    
    Note over System,ESXi: Runtime: Command Execution
    
    System->>ESXi: SSH connect with private key
    ESXi->>ESXi: Validate key
    ESXi->>Datastore: Force execute wrapper.py
    Datastore->>Datastore: Wrapper validates command<br/>against whitelist
    Datastore->>Datastore: Sanitize parameters
    Datastore->>ESXi: Execute vmkfstools
    ESXi->>System: Return task ID
    
    style K8sSecrets fill:#ffe1e1
    style Datastore fill:#fff4e1
```

---

## Usage Notes

These diagrams cover:

1. **System Context** - Overall architecture
2. **Sequence Diagram** - Complete operation flow
3. **Storage Interface** - Abstraction layer design
4. **Interface Methods** - Detailed method flows
5. **State Machine** - Task lifecycle
6. **SSH vs VIB** - Architecture comparison
7. **Datastore Mapping** - Storage array correlation
8. **Component Detail** - Internal architecture
9. **Error Recovery** - Failure handling
10. **SSH Security** - Key setup and command restriction

You can view these in any Markdown viewer that supports Mermaid diagrams (GitHub, GitLab, VS Code with extensions, etc.)
