-- This is default tarantool initialization file
-- with easy to use configuration examples including
-- replication, sharding and all major features
-- Complete documentation available in:  http://tarantool.org/doc/
--
-- To start this instance please run `systemctl start tarantool@regTrntl` or
-- use init scripts provided by binary packages.
-- To connect to the instance, use "sudo tarantoolctl enter regTrntl"
-- Features:
-- 1. Database configuration
-- 2. Binary logging and automatic checkpoints
-- 3. Replication
-- 4. Automatinc sharding
-- 5. Message queue
-- 6. Data expiration

-----------------
-- Configuration
-----------------
box.cfg {
    ------------------------
    -- Network configuration
    ------------------------

    -- The read/write data port number or URI
    -- Has no default value, so must be specified if
    -- connections will occur from remote clients
    -- that do not use “admin address”
    listen = '127.0.0.1:3301';
    -- listen = '*:3301';

    -- The server is considered to be a Tarantool replica
    -- it will try to connect to the master
    -- which replication_source specifies with a URI
    -- for example konstantin:secret_password@tarantool.org:3301
    -- by default username is "guest"
    -- replication_source="127.0.0.1:3102";

    -- The server will sleep for io_collect_interval seconds
    -- between iterations of the event loop
    io_collect_interval = nil;

    -- The size of the read-ahead buffer associated with a client connection
    readahead = 16320;

    ----------------------
    -- Memtx configuration
    ----------------------

    -- An absolute path to directory where snapshot (.snap) files are stored.
    -- If not specified, defaults to /var/lib/tarantool/INSTANCE
    -- memtx_dir = nil;

    -- How much memory Memtx engine allocates
    -- to actually store tuples, in bytes.
    memtx_memory = 128 * 1024 * 1024; -- 128Mb

    -- Size of the smallest allocation unit, in bytes.
    -- It can be tuned up if most of the tuples are not so small
    memtx_min_tuple_size = 16;

    -- Size of the largest allocation unit, in bytes.
    -- It can be tuned up if it is necessary to store large tuples
    memtx_max_tuple_size = 128 * 1024 * 1024; -- 128Mb

    -- Reduce the throttling effect of box.snapshot() on
    -- INSERT/UPDATE/DELETE performance by setting a limit
    -- on how many megabytes per second it can write to disk
    -- memtx_snap_io_rate_limit = nil;

    ----------------------
    -- Vinyl configuration
    ----------------------

    -- An absolute path to directory where Vinyl files are stored.
    -- If not specified, defaults to /var/lib/tarantool/INSTANCE
    -- vinyl_dir = nil;

    -- How much memory Vinyl engine can use for in-memory level, in bytes.
    vinyl_memory = 128 * 1024 * 1024; -- 128Mb

    -- How much memory Vinyl engine can use for caches, in bytes.
    vinyl_cache = 128 * 1024 * 1024; -- 128Mb

    -- Size of the largest allocation unit, in bytes.
    -- It can be tuned up if it is necessary to store large tuples
    vinyl_max_tuple_size = 128 * 1024 * 1024; -- 128Mb

    -- The maximum number of background workers for compaction.
    vinyl_write_threads = 2;

    ------------------------------
    -- Binary logging and recovery
    ------------------------------

    -- An absolute path to directory where write-ahead log (.xlog) files are
    -- stored. If not specified, defaults to /var/lib/tarantool/INSTANCE
    -- wal_dir = nil;

    -- Specify fiber-WAL-disk synchronization mode as:
    -- "none": write-ahead log is not maintained;
    -- "write": fibers wait for their data to be written to the write-ahead log;
    -- "fsync": fibers wait for their data, fsync follows each write;
    wal_mode = "none";

    -- The maximal size of a single write-ahead log file
    wal_max_size = 256 * 1024 * 1024;

    -- The interval between actions by the checkpoint daemon, in seconds
    checkpoint_interval = 60 * 60; -- one hour

    -- The maximum number of checkpoints that the daemon maintans
    checkpoint_count = 6;

    -- Don't abort recovery if there is an error while reading
    -- files from the disk at server start.
    force_recovery = true;

    ----------
    -- Logging
    ----------

    -- How verbose the logging is. There are six log verbosity classes:
    -- 1 – SYSERROR
    -- 2 – ERROR
    -- 3 – CRITICAL
    -- 4 – WARNING
    -- 5 – INFO
    -- 6 – VERBOSE
    -- 7 – DEBUG
    log_level = 5;

    -- By default, the log is sent to /var/log/tarantool/INSTANCE.log
    -- If logger is specified, the log is sent to the file named in the string
    -- logger = "example.log";

    -- If true, tarantool does not block on the log file descriptor
    -- when it’s not ready for write, and drops the message instead
    log_nonblock = false;

    -- If processing a request takes longer than
    -- the given value (in seconds), warn about it in the log
    too_long_threshold = 0.5;

    -- Inject the given string into server process title
    -- custom_proc_title = 'example';
}

local function bootstrap()
    local space = box.schema.create_space('auth')
    space:format({{name='login',type='string'},
    {name='password', type='string'}})
    space:create_index('primary',{parts={'login'}})
    space:create_index('secondary',{parts={{'login'},{'password'}}})

    local space2 = box.schema.create_space('regcodes')
    space2:format({{name='code',type='integer'},
    {name='hash',type='string'},
    {name='data',type='string'},
    {name='metaid',type='string'},
    {name='metasurname',type='string'},
    {name='metaname',type='string'},
    {name='password',type='string'},
    {name='role',type='integer'},
    {name='status',type='integer'}})
    space2:create_index('primary',{parts={'code'}})
    space2:create_index('secondary',{parts={'hash'}})
    space2:insert{'11fbe45e3265875e811944ccf19ec8e3', '2971fd5dda5f5a13cc3c4c380877192d'}
  - space2:insert{'21232f297a57a5a743894a0e4a801fc3', '9c42a1346e333a770904b2a2b37fa7d3'}
  - space2:insert{'2c8496da7da45f4b01325b93e5b81b7e', '09ff4dd2622902d1edef0affae11622e'}
  - space2:insert{'31244529b9438a189fbc6bbf69045468', '8120a4ab0f2bc43aa1eac1efb5874a76'}
  - space2:insert{'4220b82a7d0f3ca7212078fd3a2ff684', 'e030c39530066b7fc2684bbdc09c693d'}
  - space2:insert{'44d51eb3a8b66832a7377c65296370ee', 'bc019f734056dc385c4e21e03d615f2a'}
  - space2:insert{'565fd0dad8ff24bd955250d1ffe723dd', '2da1330bba305db17d10378a223c48e6'}
  - space2:insert{'78a654f7fad02cca509017d16539aa02', '213bc5570389208ae306bd2bdb4cb135'}
  - space2:insert{'82ce634a6332db567e50babfde4545f2', '684b5cf561e84248e31fc0cdc4b6ffc0'}
  - space2:insert{'99af3c3d0a34f7f847149fff36795672', 'd3617d36d26d3087c35e9ad68645a0dc'}
  - space2:insert{'d5123ae61159de06763e1ab6c75f7a29', 'b1748dd378866f80421cd3a306eb453b'}
  - space2:insert{'df6d90e8a18955225da2459955412f86', '19bc14570d2396a16e482cfaf19dcebd'}
  - space2:insert{'dfaa1de8e5ec7b67b724a7a46bda7f81', '79b5ff009f45a2037afd2ae7007f2ecb'}
  - space2:insert{'fdd632c82c1b346c50d514e0bcbb1d5b', '13a57d40e02967c735513ddc7cb52ea7'}
    -- Comment this if you need fine grained access control (without it, guest
    -- will have access to everything)
    box.schema.user.grant('guest', 'read,write,execute', 'universe')

    -- Keep things safe by default
    --  box.schema.user.create('example', { password = 'secret' })
    --  box.schema.user.grant('example', 'replication')
    --  box.schema.user.grant('example', 'read,write,execute', 'space', 'example')
end

-- for first run create a space and add set up grants
box.once('init', bootstrap)

-----------------------
-- Automatinc sharding
-----------------------
-- N.B. you need install tarantool-shard package to use shadring
-- Docs: https://github.com/tarantool/shard/blob/master/README.md
-- Example:
--  local shard = require('shard')
--  local shards = {
--      servers = {
--          { uri = [[host1.com:4301]]; zone = [[0]]; };
--          { uri = [[host2.com:4302]]; zone = [[1]]; };
--      };
--      login = 'tester';
--      password = 'pass';
--      redundancy = 2;
--      binary = '127.0.0.1:3301';
--      monitor = false;
--  }
--  shard.init(shards)

-----------------
-- Message queue
-----------------
-- N.B. you need to install tarantool-queue package to use queue
-- Docs: https://github.com/tarantool/queue/blob/master/README.md
-- Example:
--  local queue = require('queue')
--  queue.create_tube(tube_name, 'fifottl')

-------------------
-- Data expiration
-------------------
-- N.B. you need to install tarantool-expirationd package to use expirationd
-- Docs: https://github.com/tarantool/expirationd/blob/master/README.md
-- Example (deletion of all tuples):
--  local expirationd = require('expirationd')
--  local function is_expired(args, tuple)
--    return true
--  end
--  expirationd.start("clean_all", space.id, is_expired {
--    tuple_per_item = 50,
--    full_scan_time = 3600
--  })
