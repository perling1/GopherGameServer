package database

import (
	"errors"
	"sync"
	"database/sql"
)

var (
	shardingRules []shardingOptions = []shardingOptions{shardingOptions{}, shardingOptions{}, shardingOptions{}}
)

const (
	ShardingTableUsers = iota // The Users table is being sharded
	ShardingTableFriends      // The Friends table is being sharded
	ShardingTableAutologs     // The Autologs table is being sharded
)

const (
	ShardByLetter = iota
	ShardByNumber
)

type shardingOptions struct {
	column string // The name of the column to use for sharding

	letterShards map[string]dbShard

	interval         int
	numberMux        sync.Mutex
	numberShards     map[int]dbShard
	highestInterval  int
	newShardNumber   int
	newShardCallback func(int) error
}

type dbShard struct {
	conn *sql.DB

	ip       string
	port     int
	protocol string
	user     string
	password string
	database string
}

func SetShardingColumn (table int, column string, shardType int) error {
	if serverPaused || serverStarted {
		return errors.New("Cannot make new sharding rule type once the server has started")
	} else if table < 0 || table > len(shardingRules)-1 {
		return errors.New("Incorrect table number")
	} else if shardType != ShardByLetter && shardType != ShardByNumber {
		return errors.New("Incorrect sharding type")
	} else if shardingRules[table].numberShards != nil && shardType == ShardByLetter {
		return errors.New("Table is already using numeric sharding rules")
	} else if shardingRules[table].letterShards != nil && shardType == ShardByNumber {
		return errors.New("Table is already using letter sharding rules")
	} else if shardingRules[table].column != "" {
		return errors.New("Table already has a sharding rule type for the column '"+shardingRules[table].column+"'")
	} else if column == "" {
		return errors.New("Must supply a column name")
	}

	// Set the rule type
	shardingRules[table].column = column

	if shardType == ShardByLetter {
		shardingRules[table].letterShards = make(map[string]dbShard)
	} else if shardType == ShardByNumber {
		shardingRules[table].numberShards = make(map[int]dbShard)
		// Set defaults
		shardingRules[table].interval = 20000;
		shardingRules[table].newShardNumber = 19000;
		shardingRules[table].highestInterval = 1;
		shardingRules[table].newShardCallback = defaultNewNumberShard;
	}

	return nil
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   SHARDING BY LETTER   //////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func SetShardingLetterDatabase(table int, letter string, ip string, port int, protocol string, user string, password string, db string) error {
	if serverPaused || serverStarted {
		return errors.New("Cannot make new sharding rule by letter once the server has started")
	} else if table < 0 || table > len(shardingRules)-1 {
		return errors.New("Incorrect table number")
	} else if shardingRules[table].numberShards != nil {
		return errors.New("Table is using numeric sharding rules")
	} else if shardingRules[table].column == "" {
		return errors.New("Table has no sharding column set")
	}

	rule := dbShard{ ip: ip, port: port, protocol: protocol, user: user, password: password, database: db }

	shardingRules[table].letterShards[letter] = rule;

	return nil
}

func GetLetterDatabase(s string) {
	// Get a database by the starting letter(s) of a string
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//   SHARDING BY NUMBER   //////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// SetShardingInterval sets the interval at which a numerically sharded database will shard by.
func SetShardingInterval(table int, interval int, newShardNumber int, newShardCallback func(int) error) error {
	if serverPaused || serverStarted {
		return errors.New("Cannot set sharding interval once the server has started")
	} else if table < 0 || table > len(shardingRules)-1 {
		return errors.New("Incorrect table number")
	} else if shardingRules[table].letterShards != nil {
		return errors.New("Table is using letter sharding rules")
	} else if shardingRules[table].column == "" {
		return errors.New("Table has no sharding column set")
	} else if interval <= 100 {
		return errors.New("Sharding interval requires a minimum of 100")
	} else if newShardNumber >= interval || newShardNumber < 50 {
		return errors.New("New shard number requires a minimum of 50 and must be less than sharding interval")
	}

	shardingRules[table].interval = interval;
	shardingRules[table].highestInterval = 1;
	shardingRules[table].newShardNumber = newShardNumber;
	if newShardCallback != nil {
		shardingRules[table].newShardCallback = newShardCallback;
	}

	return nil
}

// NewNumberDatabase will open a connection to a database shard
func NewNumberShard(table int, start int, ip string, port int, protocol string, user string, password string, db string) error {
	if table < 0 || table > len(shardingRules)-1 {
		return errors.New("Incorrect table number")
	} else if shardingRules[table].letterShards != nil {
		return errors.New("Table is using letter sharding rules")
	} else if shardingRules[table].column == "" {
		return errors.New("Table has no sharding column set")
	} else if start != 1 && start % shardingRules[table].interval != 0 {
		return errors.New("Shard start number must be 1 or divisible by the sharding interval")
	}

	rule := dbShard{ ip: ip, port: port, protocol: protocol, user: user, password: password, database: db }

	shardingRules[table].numberMux.Lock()
	if start != shardingRules[table].highestInterval {
		shardingRules[table].numberMux.Unlock()
		return errors.New("New shard's start must be equal to the table's highest interval")
	}

	if start == 1 {
		shardingRules[table].highestInterval = shardingRules[table].interval
	}else{
		shardingRules[table].highestInterval = start+shardingRules[table].interval
	}
	shardingRules[table].numberShards[start] = rule
	shardingRules[table].numberMux.Unlock()

	return nil
}

func defaultNewNumberShard(table int) error {
	// make a new table on same database instance
	return nil
}