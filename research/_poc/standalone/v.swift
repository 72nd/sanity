import Vision, Foundation
let url = URL(fileURLWithPath: CommandLine.arguments[1])
let req = VNRecognizeTextRequest()
req.recognitionLevel = CommandLine.arguments.count > 2 ? .fast : .accurate
req.recognitionLanguages = ["de-DE","en-US"]
let h = VNImageRequestHandler(url: url)
do { try h.perform([req]); print(req.results?.count ?? -1); for o in req.results ?? [] { print(o.topCandidates(1).first?.string ?? "") } } catch { print("ERR", error) }
