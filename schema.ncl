# The contract config.ncl must satisfy. The app evaluates config.ncl, so a
# value that breaks this contract is reported (with where it came from)
# before the app starts.
let Port = std.contract.from_predicate (fun p => std.is_number p && p > 0 && p < 65536) in
{
  name | String | doc "What the server calls itself",
  port | Port | doc "The port to listen on" | default = 8080,
  greetings
    | { _ : String }
    | doc "A greeting for each language code, served at /<code>"
    | default
    = { en = "Hello" },
}
