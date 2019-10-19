package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gocql/gocql"
)

func fail(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format + "\n", args...)
	os.Exit(1)
}

func create_session(cluster *gocql.ClusterConfig) *gocql.Session {
	session, err := cluster.CreateSession()
	if err != nil {
		fail("Failed to create a CQL session: %s", err)
	}
	return session
}

func exec_query(session *gocql.Session, stmt string) {
	err := session.Query(stmt).Exec()
	if err != nil {
		fail("Failed to execute query: %s: %s", stmt, err)
	}
}

func run(cluster *gocql.ClusterConfig, table string, client_id int, max_val int, delay time.Duration, out chan string) {
	session := create_session(cluster)
	query := session.Query(fmt.Sprintf("UPDATE %s SET value = ? WHERE id = 0 IF value = ?", table))

	val := 1
	for val <= max_val {
		var old_val int
		applied, err := query.Bind(val + 1, val).ScanCAS(&old_val)
		var next_val int
		var status string
		if err != nil {
			// In case of a CAS error, retry with the same value.
			status = "error"
			next_val = val
		} else if applied {
			// In case CAS succeeded, retry with an incremented value.
			status = "success"
			next_val = val + 1;
		} else {
			// In case CAS failed, retry with the returned value.
			status = "fail"
			next_val = old_val;
		}
		out <- fmt.Sprintf("%d %d %s", client_id, val, status)
		val = next_val
		time.Sleep(delay)
	}
	out <- ""
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
`Usage:
  %s [options...] host

Run multiple clients that connect to the given Cassandra host and concurrently
increment  the same counter using the CAS primitive until it reaches the given
maximum.  It then prints the result of the execution to the standard output in
the format  <client> <value> <status>  where <client> is the client identifier
(integer starting from 1), <value> is the expected value of the counter passed
to CAS, <status>  is 'success' if  CAS successfully  incremented  the counter,
'fail'  if  the counter value  did not match  the expected one, or  'error' if
an error (e.g. timeout)  was returned to the client.  Before starting the test
the program creates the test table unless it already exists.

`,
			    os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}
	max_val := flag.Int("m", 500, "max value of the counter to increment")
	client_count := flag.Int("n", 5, "number of clients concurrently incrementing the counter")
	delay := flag.Duration("d", 0, "delay between successive increment operations")
	keyspace := flag.String("k", "castest", "keyspace to use for the test")
	table := flag.String("t", "castest", "table to use for the test")
	repl_factor := flag.Int("r", 3, "replication factor to use for the test keyspace")
	recreate_schema := flag.Bool("c", false, "drop and recreate the schema before starting the test")
	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		flag.Usage()
	}

	cluster := gocql.NewCluster(args[0])
	session := create_session(cluster)
	if *recreate_schema {
		exec_query(session, fmt.Sprintf("DROP KEYSPACE IF EXISTS %s", *keyspace))
	}
	exec_query(session, fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s WITH " +
					"replication = {'class': 'SimpleStrategy', 'replication_factor' : %d}",
					*keyspace, *repl_factor))
	exec_query(session, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s.%s (id int PRIMARY KEY, value int)",
					*keyspace, *table))
	exec_query(session, fmt.Sprintf("INSERT INTO %s.%s(id, value) VALUES(0, 1)", *keyspace, *table))
	session.Close()

	cluster.Keyspace = *keyspace

	out := make(chan string, *client_count * 64)
	for client_id := 1; client_id <= *client_count; client_id++ {
		go run(cluster, *table, client_id, *max_val, *delay, out)
	}
	for *client_count > 0 {
		s := <-out
		if len(s) == 0 {
			*client_count--
			continue
		}
		fmt.Println(s)
	}
}
