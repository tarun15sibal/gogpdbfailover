# Greenplum Database Monitoring Tool

This Go program monitors a Greenplum database cluster. It performs various health checks, including:
- Pinging master, standby master, and segment hosts.
- Executing queries against the database.
- Verifying Virtual IP (VIP) status.
- Activating the standby master node if the primary master is down.
It can also send email alerts based on the status of these checks.

## Prerequisites

- Go programming language environment.
- The following Go libraries (the program will fetch them automatically if they are not present):
  - `github.com/hypersleep/easyssh`
  - `github.com/lib/pq`
- A configuration file. This file must be provided as the first command-line argument when running the program. See the "Configuration File" section below for details on its format.
- SSH access with key-based authentication configured for the `gpadmin` user (or the user running the script) from the host running this tool to all Greenplum hosts (master, standby master, segments). The SSH key path is hardcoded as `/home/gpadmin/.ssh/id_rsa`.

## Functionality

The tool performs a sequence of checks and actions to monitor the Greenplum cluster:

1.  **Ping Checks:**
    *   Pings the master host (`mdw`) from the host where the tool is running.
    *   SSHes into the master host (`mdw`) and pings all segment hosts (`sdw`) listed in the configuration.
    *   If any ping fails, an email alert is sent, and the program exits.

2.  **Database Query Check (Master):**
    *   Connects to the database specified (`db`) on the master host (`mdw`).
    *   Executes a simple query (`select * from pg_stat_activity`).
    *   If the connection or query fails, an email alert is sent, and the program exits.

3.  **VIP Reachability Check:**
    *   Pings the virtual IP (`vip`) from the host where the tool is running.
    *   If the VIP is reachable (which it shouldn't be if the master is presumed down or being checked for failover), an email alert is sent, and the program exits. This check assumes the VIP should *not* be active before standby activation.

4.  **Standby Activation and Validation:**
    *   SSHes into the standby master host (`smdw`).
    *   Runs `gpactivatestandby` using the provided data directory (`datadir`).
    *   Validates the standby activation using `gpstate -f`.
    *   If activation or validation fails, an email alert is sent, and the program exits.
    *   **Database Query Check (Standby):** Connects to the database on the newly activated standby master (`smdw`) and runs a query to ensure it's operational.
    *   **VIP Activation on Standby:**
        *   Checks if the VIP is still reachable (it shouldn't be). If it is, an alert is sent as this might indicate a misconfiguration or that the master's VIP wasn't properly brought down.
        *   Activates the VIP (`ifconfig eth0:0 <vip> up`) on the standby host.
        *   If VIP activation fails, an email alert is sent, and the program exits.
    *   **Database Query Check (Standby via VIP):** Connects to the database on the standby host using the VIP and runs a query to ensure it's accessible via the VIP.
    *   If any of these sub-steps fail, an email alert is sent, and the program exits.

5.  **Email Alerts:**
    *   The program sends an email to the `mail_list` specified in the configuration if any of the checks or activation steps fail. The email subject includes the master hostname, and the body contains a message detailing the success or failure of the operations performed.

## Configuration File

The program requires a configuration file passed as its first command-line argument. The file should contain key-value pairs, with each pair on a new line, separated by a colon (`:`).

**Example Configuration File:**

```
mdw:master_hostname_or_ip
smdw:standby_master_hostname_or_ip
sdw:segment1_ip,segment2_ip,segment3_ip
vip:virtual_ip_address
db:your_database_name
datadir:/data/master/gpseg-1
mail_list:user1@example.com,user2@example.com
```

**Keys:**

*   `mdw`: The hostname or IP address of the primary master server.
*   `smdw`: The hostname or IP address of the standby master server.
*   `sdw`: A comma-separated list of segment hostnames or IP addresses. (Note: The current ping check for segments iterates through these but seems to attempt to SSH into `mdw` to ping them, which might be an issue if `mdw` is down. This behavior should be reviewed.)
*   `vip`: The virtual IP address used for master failover.
*   `db`: The name of the database to connect to for checks.
*   `datadir`: The Greenplum master data directory path on the standby server, required for `gpactivatestandby`.
*   `mail_list`: A comma-separated list of email addresses for sending alerts.

## How to Run

1.  Ensure you have Go installed and the prerequisites are met.
2.  Create a configuration file as described above.
3.  Open your terminal and navigate to the directory containing `gorealpgm.go`.
4.  Run the program using the following command:

    ```bash
    go run gorealpgm.go /path/to/your/config_file.txt
    ```

    Replace `/path/to/your/config_file.txt` with the actual path to your configuration file.

## Future Improvements / Known Issues

*   **Refine VIP Handling:** The script includes comments like "create a function to disable VIP". The logic for checking and managing the VIP, especially ensuring it's down on the master before activating on standby, could be more robust.
*   **Error Handling and Recovery:** The script currently exits on most errors. More sophisticated error handling and potential retry mechanisms could be implemented.
*   **Segment Ping Logic:** The `ping_check` function for segments (`sdw`) appears to SSH into the master (`mdw`) to ping the segments. If the master is down (a common scenario for failover), this check will fail. Pings to segments should ideally originate from the host running the tool or from the standby master.
*   **SSH Key Path:** The SSH key path (`/home/gpadmin/.ssh/id_rsa`) is hardcoded. This should be configurable.
*   **Security:** The database password (`Gpadmin1`) is visible in the `db_qry_chk` function when constructing the connection string. This should be externalized or handled more securely (e.g., environment variables, configuration file parameter).
*   **Code Clarity and Comments:** While there are some comments, adding more detailed comments explaining each function's purpose, parameters, and return values would improve maintainability.
*   **Modularity:** The code could be broken down into more specific packages or modules for better organization.
*   **Testing:** Add unit and integration tests for the various functions.
*   **Mail Server Configuration:** The mail server (`mailsyshubprd05.lss.emc.com:25`) is hardcoded. This should be configurable.
