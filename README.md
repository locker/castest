# castest
Utility to check CAS consistency of a Cassandra cluster

## About

The utility runs multiple clients that connect to the given Cassandra host and
concurrently increment the same counter using the CAS primitive until it reaches
the given maximum. It then prints the result of the execution to the standard
output in the format `client value status` where `client` is the client
identifier (integer starting from 1), `value` is the expected value of the
counter passed to CAS, `status` is `success` if CAS successfully incremented the
counter, `fail` if the counter value did not match the expected one, or `error`
if an error (e.g. timeout) was returned to the client. Before starting the test
the program creates the test table unless it already exists.

## Building

Enter the project directory and run:

```
$ go get github.com/gocql/gocq
$ go build castest.go
```

## Running

First, create and start a cassandra cluster (e.g. using ccm), then run:

```
$ ./castest 127.0.0.1
```

By default the program creates a test keyspace with replication factor set to 3.
To use a different replication factor, use `-r` option. To force recreation of
the test keyspace before running the test, also pass `-c`. For example, to rerun
the test against a keyspace with replication factor of 1, run:

```
$ ./castest -r 1 -c 127.0.0.1
```

To change the number of clients or the number of iterations, use `-n` and `-m`
options. For example, to run the 10 clients concurrently incrementing the
counter up to 5000, run:

```
$ ./castest -n 10 -m 5000 127.0.0.1
```

## Checking consistency

To check CAS consistency, feed the output of a `castest` execution to the
`cascheck` script:

```
$ ./castest 127.0.0.1 | ./cascheck
```

The script will print a summary about the execution:

 - Number of successful CAS executions, number of CAS failures, number of errors
   occurred.
 - Min and max value of the counter observed by the clients.
 - Gaps, i.e. values that haven't been incremented successfully by any of the
   clients. This may happen without violating consistency, in case a client
   successfully increments the counter, but is returned an error (e.g. timeout).
 - Duplicates, i.e. values that have been successfully incremented by more than
   one client. This is a clear consistency violation.
 - Disordered operations, i.e. a lesser value seen after a greater value by the
   same client. Again, this is a clear consistency violation.
