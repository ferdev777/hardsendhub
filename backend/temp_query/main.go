package main
import ("database/sql"
"fmt"
_ "modernc.org/sqlite")
func main() {db, _ := sql.Open("sqlite", "../database.sqlite"); var c int; db.QueryRow("SELECT count(*) FROM blacklist").Scan(&c); fmt.Println("Baneados:", c)}
