# BlockEmulator

## Introduction

> **To provide more standardized code, facilitate user-side secondary development, and reduce potential bugs,
BlockEmulator underwent a refactoring in 2025: BlockEmulator-X.**

> **This document outlines the startup method, design principles,
and major updates of the new version of BlockEmulator (BlockEmulator-X).**

> **The document of BlockEmulator v1.0 can be found in https://github.com/HuangLab-SYSU/block-emulator.**

### Background

Initiated by **[HuangLab](http://xintelligence.pro/)**(a research group
in the School of Software Engineering, Sun Yat-sen University, China),
**BlockEmulator** is a blockchain testbed that enables researchers to
verify their proposed new protocols and mechanisms.
It supports multiple consensus protocols, particularly the blockchain sharding mechanism.

The main purpose of this testbed is to help users (researchers, students, etc.)
quickly verify their own blockchain consensus protocols and blockchain-sharding protocols.

**BlockEmulator** is designed as an experimental platform that adopts a lightweight system architecture.
It simplifies the implementation of industrial-class blockchains
since **BlockEmulator** only implements the core functions of a blockchain,
including the transaction pool, block packaging, consensus protocols, and on-chain transaction storage.
It also supports common consensus protocols, such as Practical Byzantine Fault Tolerance (PBFT).

In particular, BlockEmulator offers the system-level design and implementation for blockchain-sharding mechanisms.
For example, the cross-shard transaction mechanisms implemented by BlockEmulator
include the following two representative solutions:
i) **Relay transaction mechanism** proposed by **Monoxide** (NSDI'2019), and
ii) the **BrokerChain** protocol (INFOCOM'2022)
[PDF](https://www.researchgate.net/publication/356789473_BrokerChain_A_Cross-Shard_Blockchain_Protocol_for_AccountBalance-based_State_Sharding).

BlockEmulator is oriented toward blockchain researchers.
It provides a blockchain experimental platform for quickly implementing their own algorithms,
protocols, and mechanisms. It also offers very helpful functions to help researchers collect
experimental data, facilitating their plotting experimental figures.

### Official Technical Paper & Citation

To provide an official handbook for BlockEmulator,
we have written a technical paper titled
"BlockEmulator: An Emulator Enabling to Test Blockchain Sharding Protocols"
[arXiv page](https://arxiv.org/abs/2311.03612).
Please cite our paper if you use BlockEmulator as an experiment tool in your own paper.

```
@article{huang2025blockemulator,
   title={BlockEmulator: An Emulator Enabling to Test Blockchain Sharding Protocols},
   author={Huang, Huawei and Ye, Guang and Yang, Qinglin and Chen, Qinde and Yin, Zhaokang and Luo, Xiaofei and Lin, Jianru and Zheng, Jian and Li, Taotao and  Zheng, Zibin},
   journal = {IEEE Transactions on Services Computing (TSC)},
   volume={18},
   number={2},
   pages = {690--703},
   year = {2025},
   }
```

### Related Work

The following papers from HuangLab's publications have adopted **BlockEmulator** as an experimental tool.

1. **BrokerChain**: A Cross-Shard Blockchain Protocol for Account/Balance-based State Sharding **(INFOCOM 2022)**
   [PDF](https://www.researchgate.net/publication/356789473_BrokerChain_A_Cross-Shard_Blockchain_Protocol_for_AccountBalance-based_State_Sharding)

2. **Broker2Earn**: Towards Maximizing Broker Revenue and System Liquidity for Sharded Blockchains **(INFOCOM 2024)**
   [PDF](https://www.researchgate.net/publication/379213048_Broker2Earn_Towards_Maximizing_Broker_Revenue_and_System_Liquidity_for_Sharded_Blockchains)

3. Account Migration across Blockchain Shards using Fine-tuned Lock Mechanism **(INFOCOM 2024)**
   [PDF](https://www.researchgate.net/publication/379210418_Account_Migration_across_Blockchain_Shards_using_Fine-tuned_Lock_Mechanism)

4. **Justitia**: An Incentive Mechanism towards the Fairness of Cross-shard Transactions **(INFOCOM 2025)**
   [PDF](http://xintelligence.pro/archives/1371)

5. Scheduling Most Valuable Committees for the Sharded Blockchain **(ToN 2023)**
   [PDF](https://www.researchgate.net/publication/370671128_Scheduling_Most_Valuable_Committees_for_the_Sharded_Blockchain)

6. Achieving Scalability and Load Balance across Blockchain Shards for State Sharding (published at SRDS 2022)
   [PDF](https://ieeexplore.ieee.org/document/9996899)

7. **tMPT**: Reconfiguration across Blockchain Shards via Trimmed Merkle Patricia Trie (published at IWQoS 2023)
   [PDF](https://www.researchgate.net/publication/370633426_tMPT_Reconfiguration_across_Blockchain_Shards_via_Trimmed_Merkle_Patricia_Trie)

8. **MVCom**: Scheduling Most Valuable Committees for the Large-Scale Sharded Blockchain (published at ICDCS 2021)
   [PDF](https://ieeexplore.ieee.org/document/9546408)

### Highlights of BlockEmulator

1. **Lightweight**. BlockEmulator is a lightweight testbed platform for blockchain experiments.

2. **Fast Configuration**. BlockEmulator enables users to set up their environments quickly
   and supports remote deployment on the Cloud.

3. **Customization**. BlockEmulator is implemented using the GoLand language,
   which supports users' customization and modification.

4. **Easy to Conduct Experiments**. BlockEmulator supports the replay of historical
   transactions of mainstream blockchains (such as Ethereum).
   It can automatically yield experimental log files. Using those log files,
   researchers can interpret plenty of metrics such as system throughput,
   confirmation latency of transactions, the queueing of the transaction pool, etc.
   This function is very useful for researchers and students to
   facilitate their experimental data collection and plotting of experimental charts.

## Start

### Running Basic-Scale Experiments

BlockEmulator v2.0 includes a startup script for basic-scale experiments (`example_run.sh`),
which launches a setup with 4 shards, 4 nodes per shard, plus one Supervisor.
This script can also automatically download dependencies, remove historical data, and compile the code.

The basic-scale script is:

```sh
#!/bin/bash

SHARD_NUM=4
NODE_NUM=4

# Delete the old experiment directory.
rm -rf ./exp/
mkdir -p ./exp/

set -ex

# Download modules and pre-compile.
go mod download
go build ./...

# Start consensus nodes.
for ((i=0; i<SHARD_NUM; i++)); do
  for ((j=0; j<NODE_NUM; j++)); do
    go run cmd/consensusnode/main.go -shard_id="${i}" -node_id="${j}" &
  done
done

# Start the supervisor.
go run cmd/supervisor/main.go -shard_id=0x7fffffff -node_id=0 &

wait
```

This script can be start by using `bash`:

```sh
bash example_run.sh
```

### Running Experiments of a Specified Scale

The script mentioned above only supports launching a basic-scale blockchain system.
To run experiments with larger scales or different consensus protocols, you need to configure BlockEmulator:

#### Configure `config.yaml`

Users can customize the system by editing the `config.yaml` file,
where each configuration item is described in detail.

Configurable parameters include, but are not limited to:

- Number of shards (`system.shard_num`)

- Number of nodes per shard (`system.node_num`)

- Consensus protocol (`system.consensus_type`)

- Total number of injected transactions (`supervisor.tx_number`)

- Transaction injection rate (`supervisor.tx_injection_speed`)

- ...

After modifying the configuration, the settings will take effect the next time the nodes are launched.

When performing secondary development,
if users add new features, they can also add corresponding configuration entries to `config.yaml`.

#### Configuring the IP Table

If the user modifies the default number of shards (`system.shard_num`) or
the number of nodes per shard (`system.node_num`),
they must ensure that the `ip_table.json` file in the root directory contains the IP addresses for all nodes.

Below is an example of `ip_table.json` for a system with 2 shards and 2 nodes per shard:

```json
{
  "0": {
    "0": "127.0.0.1:32217",
    "1": "127.0.0.1:32227"
  },
  "1": {
    "0": "127.0.0.1:32317",
    "1": "127.0.0.1:32327"
  },
  "2147483647": {
    "0": "127.0.0.1:38800"
  }
}
```

#### Modifying example_run.sh

If the user changes the default number of shards (`system.shard_num`) or
the number of nodes per shard (`system.node_num`),
they must also update the variables `SHARD_NUM` and `NODE_NUM` in `example_run.sh.`
After making the changes, simply run the script.

For secondary development or for integrating BlockEmulator into other systems,
users may also write their own custom startup scripts according to their needs.

## System Architecture

In BlockEmulator, **nodes are divided into _Supervisor_ and _ConsensusNode (called Worker in v1.0)_**.

1. **Supervisor**:
   Responsible for **sending transactions**, **functioning as the committee**,
   and **collecting/aggregating system metrics**. In each BlockEmulator experiment,
   there is exactly one Supervisor in the system.

2. **ConsensusNode**:
   Responsible for **block production and consensus**.
   In a sharded blockchain, the system contains multiple shards, and each shard consists of several ConsensusNodes.

    1. ConsensusNodes within the same shard perform **inner-shard consensus** to agree on block production
       and jointly maintain a blockchain.

    2. ConsensusNodes in different shards perform **cross-shard consensus** to exchange cross-shard messages,
       such as **transaction relay** or **account migration**.

![The system architecture of BlockEmulator](docs/figures/svgs/system-architecture.svg)

**Figure: The system architecture of BlockEmulator, with the example deployment scale being 4×4+1 (4 shards,
4 nodes per shard, plus one Supervisor).**

## Node Execution Flow

### Supervisor Execution Flow

When running `cmd/supervisor/main.go`, the Supervisor follows the execution flow below:

1. Initialization

    - Read command-line arguments (load `LocalParams`)

    - Read the configuration file (load `Config`)

2. Network Configuration

    - Load the IP table

    - Build the mapping from node IDs to IP addresses

    - Start the gRPC listener and initialize the message pool

3. Execution

   The Supervisor enters the `Start` function:

    1. Launch two threads: a **main thread** and a **sub-thread**.
       The sub-thread handles metric collection and runs concurrently **without data conflicts**.

    2. The main thread reads messages from the gRPC message pool.
       Each message is consumed by **both** the main thread and the sub-thread:

        - **Main thread**: Executes corresponding update operations
          depending on enabled components (e.g., CLPA module, broker module)

        - **Sub-thread**: Uses the `Measure` interface and calls `UpdateMeasureRecord`

    3. The main thread performs **Committee**-related operations (e.g., running CLPA)

    4. The main thread performs **Client** operations (i.e., dispatching transactions)

    5. Check the stop signal:

        - `false`: main thread proceeds to the next iteration

        - `true`: wait for resources to shut down, then exit

4. Quit

    - Close all remaining resources (metric output files, network connections, etc.)

![The execution flow of the supervisor](docs/figures/svgs/supervisor-exec-flow.svg)

**Figure: The execution flow of the supervisor.**

### ConsensusNode Execution Flow

When running `cmd/consensusnode/main.go`, the ConsensusNode follows the execution flow below:

1. Initialization

    - Read command-line arguments (load `LocalParams`)

    - Read the configuration file (load `Config`)

2. Network Configuration

    - Load the IP table

    - Build the mapping from node IDs to IP addresses

    - Start the gRPC listener and initialize the message pool

3. Execution

   The ConsensusNode enters the `Start` function:

    - Register message-handling functions via `RegisterHandleFunc`

    - Read messages from the gRPC message pool

    - Process messages using the `ShardInsideOp` and `ShardOutsideMsgHandler` interfaces

    - Update PBFT state based on received messages

    - The leader checks whether the conditions to issue a Propose are met;
      if so, it creates a proposal via `ShardInsideOp.BuildProposal` and broadcasts it

4. Quit
    - Close remaining resources (blockchain, network, etc.)

![The execution flow of the consensus nodes](docs/figures/svgs/consensusnode-exec-flow.svg)

**Figure: The execution flow of the consensus nodes.**

## BlockEmulator v2.0 Updates

### Configuration Items

In both BlockEmulator v1.0 and v2.0, configuration items are divided into **global configuration** and **local
parameters**:

- **Global Configuration**:
  **Shared by all nodes in the blockchain system**.
  Includes items such as the number of shards, block size, etc.

- **Local Parameters**:
  **Configurations that vary across individual nodes**.
  Includes shard ID, node ID, miner address, and more.

Compared with v1.0, BlockEmulator v2.0 uses these configuration items in a more standardized manner:
**If a data structure requires specific configuration values, it receives these parameters during creation,
rather than fetching them from global variables at runtime.**

**This reduces redundant parameters and lowers maintenance overhead.**

#### Reading Global Configuration from YAML

> **BlockEmulator v1.0**:
> Global configuration was loaded from a JSON file, but JSON does not support comments.
> As a result, users often **had difficulty understanding the meanings of specific configuration items**.

In v2.0, BlockEmulator reads global configuration from a YAML file, which does support comments.
Users can freely modify the configuration according to their needs.

BlockEmulator v2.0 provides a **default configuration file, `config.yaml`**,
which **includes annotations and value ranges for all global configuration items**.

A part of the default YAML file:

```yaml
system:
  ### -----------------------------------------------------------------------------------
  ### `shard_num` is the number of shards. It should be a positive integer.
  shard_num: 4
  ### -----------------------------------------------------------------------------------

  ### -----------------------------------------------------------------------------------
  ### `node_num` is the number of nodes per shard. It should be a positive integer.
  node_num: 4
  ### -----------------------------------------------------------------------------------
```

#### Providing Local Parameters via Command Line

BlockEmulator v2.0 follows the same approach as v1.0,
where local parameters are provided through command-line arguments.

Users can view all supported command-line parameters using the `-h` option:

```bash
# See the supported command-line arguments in ConsensusNode.
go run cmd/consensusnode/main.go -h
# Output results:
#  -account_addr string
#        miner address
#  -config string
#        path to config file (default "config.yaml")
#  -ip_table string
#        path to ip_table.json (default "ip_table.json")
#  -node_id int
#        local node id (default -1)
#  -pprof-port int
#        port to serve pprof; the port should be larger than 5000
#  -shard_id int
#        local shard id, 0x7fffffff denotes the supervisor shard (default -1)

# See the supported command-line arguments in Supervisor.
go run cmd/supervisor/main.go -h
# Outputs are the same as the above.
```

### Storage

> The storage design pattern of BlockEmulator v1.0
> dividing on-chain storage into **block storage** and **account state storage**:
> - **Block Storage**:
    **Stores all blocks belonging to the shard’s blockchain** and
    provides the ability to **retrieve blocks by their block hash**.
> - **Account State Storage**:
    Organizes account states using an **MPT (Merkle Patricia Tree)** and
    preserves **historical snapshots**, enabling fast rollback.

To better support **EVM execution** on BlockEmulator, BlockEmulator v2.0 revises the storage architecture from v1.0.
While retaining **block storage**, it splits the **account state storage** into
**two separate Merkle Patricia Tries (MPTs)**, one for **basic account state** and another for **account location**:

- **Block Storage**:
  **Stores all blocks belonging to the shard’s blockchain** and
  provides the ability to **retrieve blocks by their block hash**.

- **Basic Account State Storage**:
  Holds the fundamental account state (Balance, Nonce, Code, and StorageRoot) consistent with Ethereum.
  This part leverages Geth’s `StateDB` as its underlying implementation, **enabling full EVM compatibility**.
  Internally, `StateDB` organizes data into an MPT,
  **providing built-in support for historical snapshots and efficient state rollback**.

- **Account Location Storage**:
  Records the shard location (`ShardLocation`) of each account,
  which in a sharded blockchain identifies the shard where the account resides.
  This data is also structured as an MPT to **maintain historical snapshots and enable fast rollback**,
  ensuring consistency with the overall state versioning model.

#### Block Storage

The block storage component is implemented in `pkg/storage/block`,
and the default underlying database is BoltDB (inherited from v1.0).

It provides the following interfaces:

```go
package block

import (
    "context"
)

// Store should be a key-value database that stores the
// information of blocks.
type Store interface {
    // AddBlock adds a block into the database. It contains the operations of
    // (1) updating the newest blockHash,
    // (2) adding the header of this block into the storage,
    // (3) adding the block into the storage.
    // These 3 operations must be atomic.
    AddBlock(ctx context.Context, blockHash, encodedBlock, encodedBlockHeader []byte) error
    // GetBlockByHash returns the block with the given Hash.
    GetBlockByHash(ctx context.Context, blockHash []byte) ([]byte, error)

    // AddBlockHeader adds the header of a block into the database. It contains the operations of
    // (1) updating the newest blockHash,
    // (2) adding the header of this block into the storage.
    // These 2 operations must be atomic. Please distinguish it from AddBlock.
    // If your storage is limited, AddBlockHeader helps you catch up with other nodes quickly
    // because it reduces the storage of Block.
    AddBlockHeader(ctx context.Context, blockHash, encodedBlockHeader []byte) error
    // GetBlockHeaderByHash gets the block header according to its blockHash.
    GetBlockHeaderByHash(ctx context.Context, blockHash []byte) ([]byte, error)

    // UpdateNewestBlockHash updates the newest blockHash.
    // This function should be called when the blockchain wants to rollback.
    UpdateNewestBlockHash(ctx context.Context, newBlockHash []byte) error
    // GetNewestBlockHash gets the newest blockHash.
    // Blockchain can quickly find the tail of a chain.
    GetNewestBlockHash(ctx context.Context) ([]byte, error)

    Close() error
}
```

#### Basic Account State Storage

The basic account state storage reuses [Geth’s `StateDB`](github.com/ethereum/go-ethereum/core/state).
In BlockEmulator, the files `pkg/storage/vmstate/vmstate.go` and `pkg/vm/vm.go`
provide the wrapper logic to initialize and manage the underlying database for `StateDB`,
as well as to create and configure the `StateDB` instance itself.

```go
// pkg/storage/vmstate/vmstate.go
// NewStateStore creates state.Database for stateDB.
func NewStateStore(cfg config.StorageCfg, lp config.LocalParams) (state.Database, error)

// pkg/vm/vm.go
// NewExecutor creates a new executor with given parameters.
func NewExecutor(stateStore state.Database, root common.Hash, vmChainCfg *params.ChainConfig) (*Executor, error) {
    // Init state db.
    stateDB, err := state.New(root, stateStore)
    if err != nil {
       return nil, fmt.Errorf("failed to new a state database: %w", err)
    }

    // Set the evmCfg config for evmCfg.
    evmCfg := gethvm.Config{
       ExtraEips: []int{EIP3855},
    }

    return &Executor{
       stateDB:    stateDB,
       vmChainCfg: vmChainCfg,
       evmCfg:     evmCfg,
    }, nil
}
```

It should be noted that in the code above, `State.Database` should be created only once,
because it corresponds to the underlying database and snapshot layer.

In contrast, `StateDB` is **not reusable** after calling `Commit()`; **it must be recreated for subsequent operations**.
Typically, a new `StateDB` is instantiated for processing each block:
once the block execution is complete, the `StateDB.Commit()` is called,
and when processing the next block begins, another new `StateDB` should be created based on the updated root hash.

#### Account Location Storage

The storage of account locations is implemented in `pkg/storage/trie`.
BlockEmulator builds this component based on the `go-ethereum` codebase (version v1.16.7).
For generality, the interface is designed as a key-value (KV) storage.

BlockEmulator v2.0 splits the operation "adding account states" into two separate interfaces:
`MAddKeyValuesAndCommit` and `MAddKeyValuesPreview`:

- `MAddKeyValuesAndCommit`:
  Adds multiple {key, value} pairs, returns the updated state, and **commits** the changes to the database.

- `MAddKeyValuesPreview`:
  Adds multiple {key, value} pairs and returns the updated state, but **does not update** the database.
  This method is typically used for **validation or block generation**.

```go
// Store is an MPT-based, append-only structure whose leaf nodes should be considered as accounts.
// The upstream layer of storage not only stores nodes, but provides proofs for the nodes.
type Store interface {
   // GetCurrentRoot returns the root of the trie.
   GetCurrentRoot(ctx context.Context) ([]byte, error)
   // MGetValsByKeys returns the corresponding values with the given keys.
   MGetValsByKeys(ctx context.Context, keys [][]byte) ([][]byte, error)
   // MAddKeyValuesAndCommit adds the given key-value pairs into the trie and commits them into the database.
   MAddKeyValuesAndCommit(ctx context.Context, keys, values [][]byte) ([]byte, error)
   // MAddKeyValuesPreview adds the given key-value pairs into the trie but does not commit them.
   MAddKeyValuesPreview(ctx context.Context, keys, values [][]byte) ([]byte, error)
   // SetStateRoot sets the root of the trie.
   SetStateRoot(ctx context.Context, root []byte) error
   // Close closes the database
   Close() error
}
```

### Data Structures

BlockEmulator implements the fundamental data structures of a blockchain system in `pkg/core`,
and builds the blockchain layer (`pkg/chain`) on top of them.

### Account State

The account state structure in BlockEmulator v2.0 is adapted for a sharded blockchain system.
Compared with Ethereum, it introduces an additional field, `ShardLocation`, which indicates the shard to which an
account belongs.

> Q: How is the shard of an account determined?  
> A:   
> If the account does not exist in the database, its shard is assigned using account-address modulo.  
> If the account already exists in the database, its shard is determined by the `ShardLocation` field in its stored
> account state.

### Transaction

To support cross-shard transactions in a sharded blockchain system,
BlockEmulator v2.0 modifies the transaction structure and adds two optional fields:

```go
type Transaction struct {
   Sender     account.Address
   Recipient  account.Address
   Value      *big.Int
   Nonce      uint64
   Signature  Signature
   CreateTime time.Time
   Data        []byte
   
   GasLimit uint64
   
   RelayTxOpt  // the optional setting only for relay transactions.
   BrokerTxOpt // the optional setting only for broker transactions.
}
```

- **RelayTxOpt**:
  This structure becomes active when the system uses the Relay mechanism to process cross-shard transactions. It
  includes:

    - `RelayStage`: The stage of the relay transaction, including undefined, processing first half, and processing
      second half.

    - `ROriginalHash`: The original hash of the relay transaction; empty if it is not a cross-shard transaction.

- **BrokerTxOpt**:
  This structure becomes active when the system uses a Broker-account-based approach to process cross-shard
  transactions. It includes:

    - `BrokerStage`: The stage of the broker transaction, including non-broker transaction, broker1 transaction, and
      broker2 transaction.

    - `Broker`: The address of the broker account.

    - `BOriginalHash`: The original hash of the broker transaction; empty if it is not a cross-shard transaction.

    - Other fields mentioned in related papers but not implemented in detail.

### Block

To support account migration in a sharded blockchain system (i.e., moving an account from one shard to another),
BlockEmulator v2.0 modifies the conventional block structure and divides blocks into two types:
**Transaction Blocks (TxBlock)** and **Migration Blocks (MigrationBlock)**:

- **Transaction Block (TxBlock)**:
  Contains transactions and is used for normal transaction processing, functioning the same as traditional blockchain
  blocks.

- **Migration Block (MigrationBlock)**:
  Contains account-state information and is specifically used during the account-migration phase.

Moreover, to support **locating the shard to which an account belongs**,
BlockEmulator v2.0 introduces `LocStorage` for record account locations.
To enable **recording**, **querying**, and **rolling back** this storage, a new field called `LocationRoot`
has been added to the block header, serving as the Merkle root of the MPT for `LocStorage`.

The structure of `Block`:

```go
type Header struct {
    ParentBlockHash []byte
    StateRoot       []byte
    Number          uint64
    Miner           account.Address
    CreateTime      time.Time
	
   // LocationRoot is only used in a sharded blockchain system to denotes the root of location trie.
   // This trie is used to store the account location.
   LocationRoot []byte
    
	TxHeaderOpt
    MigrationHeaderOpt
}


// TxHeaderOpt is the struct for transaction handling.
// This struct should be used when this block is a normal one (not a block for account migration).
type TxHeaderOpt struct {
    TxRoot []byte
    Bloom  bloom.Filter
}

// MigrationHeaderOpt is the struct for the account migration.
// This struct should be used when this block is an account migration one.
type MigrationHeaderOpt struct {
    MigratedAccountsRoot []byte // MigratedAccountsRoot is the merkle root of MigratedAccounts in MigrationOpt.
}

// Body is the struct for transaction handling.
// Note that either MigrationOpt or Body is nil.
type Body struct {
    TxList []transaction.Transaction
}

// MigrationOpt is the struct for account migration.
// It saves the information of accounts that are to be migrated to this shard.
// Note that either MigrationOpt or Body is nil.
type MigrationOpt struct {
    MigratedAccounts []account.Address // MigratedAccounts is the list of accounts to be migrated in this stage.
    MigratedStates   []account.State   // MigratedStates is the list of account states corresponding to accounts in MigratedAccounts.
}

type Block struct {
    Header
    Body
    MigrationOpt
}
```

The contents of `TxHeaderOpt` are computed from the `Body`, and `MigrateHeaderOpt` is computed from `MigrateOpt`,
as shown in the figure:

![The Generation of BlockOpts](docs/figures/svgs/blockopt-generation.svg)

**Figure: The Generation of the two BlockOpts.**

### Transaction Pool

The transaction pool mainly provides the following interfaces:

```go
// TxPool is a pool that buffers transactions.
type TxPool interface {
    // AddTxs adds the given transactions into the pool.
    AddTxs(txs []transaction.Transaction) error
    // PackTxs pops transactions from the pool.
    // The size of transactions will be limited by the given parameter 'limit'. 
    PackTxs(limit int) ([]transaction.Transaction, error)
    // GetTxListSize returns the size of the given tx list.
    GetTxListSize(txs []transaction.Transaction) (int, error)
}
```

### Blockchain

Based on the storage design and basic data structures described above,
the following `Chain` structure can be defined:

```go
// Chain describes a blockchain.
type Chain struct {
    s         *storage.Storage // the storage for both block-storage and trie-storage.
    curHeader block.Header     // the current header in this blockchain.
    shardID   int64
    epochID   int64

    cfg config.BlockchainCfg

    mux sync.Mutex
}
```

The Chain structure contains many public functions, which will not be elaborated here.
One important note is:

- **Public functions must be accessed with locking**;

- **Internal functions generally do not require locks**.

### Network

> In BlockEmulator v1.0, when sending messages, the system first retrieved the target node’s IP address
> from a global IP table, and then called the TCPDial function to send the message.
> This design **limited extensibility for message-sending** (as it required explicit IP addresses) and
> **lacked message buffering capability**.

BlockEmulator v2.0 refactors the network module.
To support this change, a new node information structure is introduced in `pkg/nodetopo`:

```go
type NodeInfo struct {
    NodeID, ShardID int64
}
```

Above the network layer, programs can now send messages using the node’s `{NodeID, ShardID}` rather than raw IP
addresses.

#### Connection & Communication

When using the network module in BlockEmulator v2.0, only the target node’s NodeInfo is needed to send a message.
The interfaces for this part are as follows:

```go
// P2PConn is a peer-to-peer connection that should contain a message buffer.
type P2PConn interface {
    // ListenStart starts to listen to messages from other nodes as a server.
    ListenStart() error
    // DrainMsgBuffer drains (reads all and pops) messages in the buffer.
    DrainMsgBuffer() []*rpcserver.WrappedMsg
    // SendMsg2Dest sends the given message to the given dest node.
    SendMsg2Dest(ctx context.Context, dest nodetopo.NodeInfo, msg *rpcserver.WrappedMsg)
    Close()
}
```

`P2PConn` needs to **implement an internal message buffer to temporarily store received messages**.
The execution flow of `P2PConn` is as follows:

1. The program launches a **sub-thread** to run `ListenStart`, which listens for incoming messages from other nodes.
   `ListenStart` adds all received messages into the internal message buffer.

2. When the upper-layer code needs to read messages from the buffer, it calls `DrainMsgBuffer`,
   which retrieves all messages currently stored in the buffer.

3. When a message needs to be sent to a specific node, the program calls `SendMsg2Dest`,
   which sends the message to the target node based on the given `NodeInfo`.

![The workflow of P2PConn](docs/figures/svgs/workflow-p2pconn.svg)

**Figure: The workflow of the interface P2PConn.**

Since broadcast operations are frequently used in blockchain systems,
BlockEmulator v2.0 encapsulates these operations into the `ConnHandler` structure:

- `MSendDifferentMessages`: Sends different messages to different nodes based on the mapping `node2Msg`.

- `GroupBroadcastMessage`: Broadcasts the same message to all nodes in the given `group` array.

```go
type ConnHandler struct {
    P2PConn
}

func (p *ConnHandler) MSendDifferentMessages(ctx context.Context, node2Msg map[nodetopo.NodeInfo]*rpcserver.WrappedMsg)
func (p *ConnHandler) GroupBroadcastMessage(ctx context.Context, group []nodetopo.NodeInfo, msg *rpcserver.WrappedMsg)
```

#### Message System

BlockEmulator v2.0 uses `protobuf` (https://protobuf.dev/) to generate the RPC-related code.
**All messages transmitted in the system use the generated `WrappedMsg` structure as the carrier**:

```go
type WrappedMsg struct {
   state         protoimpl.MessageState `protogen:"open.v1"`
   MsgType       string                 `protobuf:"bytes,1,opt,name=msgType,proto3" json:"msgType,omitempty"`
   Payload       []byte                 `protobuf:"bytes,2,opt,name=payload,proto3" json:"payload,omitempty"`
   unknownFields protoimpl.UnknownFields
   sizeCache     protoimpl.SizeCache
}
```

The two key fields are:

- `MsgType`:
  The message type, which tells the upper-layer program how to decode this `WrappedMsg`.

- `Payload`:
  The message payload, containing the encoded byte sequence of various message types.

When a node processes an incoming message, it must first decode the `Payload` into
the correct message type based on `MsgType`, and then handle it.

When a node sends a message, it must encode the message into a byte sequence (`Payload`),
fill in the appropriate `MsgType`, and then send it.

![The transfer logic between `Payload` and `WrappedMsg`](docs/figures/svgs/msg-payload-transfer.svg)

**Figure: The transfer logic between `Payload` and `WrappedMsg`.**

In BlockEmulator v2.0, **decoding a `WrappedMsg` back into its corresponding message type is not supported**.
This is because implementing such an interface in Go requires extensive use of type assertions, which is considered
inelegant.

However, the file `pkg/message/message.go` does **provide the functionality for packing a message into a `WrappedMsg`**.

```go
// WrapMsg encodes different types of messages.
func WrapMsg(msg any) (*rpcserver.WrappedMsg, error) {
    msgType, err := getMsgType(msg)
    if err != nil {
       return nil, fmt.Errorf("getMsgType failed: %w", err)
    }

    var buf bytes.Buffer

    encoder := gob.NewEncoder(&buf)

    if err = encoder.Encode(msg); err != nil {
       return nil, fmt.Errorf("encoder failed: %w", err)
    }

    return &rpcserver.WrappedMsg{
       MsgType: msgType,
       Payload: buf.Bytes(),
    }, nil
}

func getMsgType(msg any) (string, error)
```

### Node Execution

> BlockEmulator v1.0 adopts a message-driven model for consensus.
> Whenever a node receives a message, it immediately spawns a new goroutine to process it.
> If the program determines that it is not the appropriate time to handle the message,
> the goroutine will sleep and wait to be awakened later.

In BlockEmulator v2.0, when a node receives a message,
it first places the message into a **message buffer**.
The node then continuously fetches messages from the buffer and processes them in order.
Details of the execution workflow for each type of node in v2.0 can be found
in [Node Execution Flow](#Node-Execution-Flow).

Compared to v1.0, BlockEmulator v2.0 uses only a single goroutine to **pop messages from the buffer and process them
sequentially**.
This ensures that **messages are handled in a strictly serialized manner,
preventing data races and other concurrency issues caused by multithreaded competition**.

![The execution difference between BlockEmulator v1.0 and v2.0](docs/figures/svgs/exec-diff-v1v2.svg)

**Figure: The execution difference between BlockEmulator v1.0 and v2.0.**

### Logging

> In BlockEmulator v1.0, log outputs were a mixture of `fmt.Print` and the consensus-shard logging modules
> (`consensus_shard/pbft_all/pbft_log` or `supervisor/supervisor_log`).
> This resulted in disorganized log messages, and only logs from the consensus layer could be written to files.

> Additionally, BlockEmulator v1.0 handled errors in a crude manner
> by calling `log.Panic()` to terminate the program directly,
> which could cause experiments to stop unexpectedly due to non-critical issues.

BlockEmulator v2.0 uses Go’s official standard library `log/slog` for logging.
It defines four log levels (Debug, Info, Warn, Error), allowing the system to print messages with different severity
levels depending on the nature of the error or exception.

**The logging mechanisms—such as printing rules and output destinations—are defined in `pkg/logger`**.
Detailed usage guidelines can be found in the comments of the `config.yaml` configuration file.