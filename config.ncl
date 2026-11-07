# The app's configuration. Imports are found relative to this file's folder.
(import "schema.ncl")
& {
  name = "example",
  greetings = {
    en = "Hello",
    fr = "Bonjour",
    pt = "Olá",
  },
}
