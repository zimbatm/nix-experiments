class SourceFetcher:
    def get_sha256():
        pass

class SourceUpdater:
    def list_versions(self):
        pass

    def get_version(self, version):
        pass

class CustomUpdater:
    def list_versions(self):
        pass

    def get_version(self, version):
        pass

FETCHER_MAP = {
    "github": GithubFetcher,
    "url": UrlFetcher,
}

UPDATER_MAP = {
    "github": GithubSourceUpdater,
    "custom": CustomSourceUpdater,
}

def fromJSON(file):
    json = jsons.load(open(file, "r"))
    fetcher = FETCHER_MAP[json.fetcher]


# little heuristic to find the src.json based only on the filename
def path_to_src_json(path):
    if path.endswith(".src.json"):
        return path
    if path.endswith(".nix"):
        return path[:-4] + ".src.json"
    if not path.endswith("/"):
        path += "/"
    return path + "default.src.json"

# -----

class SemVerion:
    def __init__(self, major, minor, tiny, meta):
        self.major = major
        self.minor = minor
        self.tiny = tiny
        self.meta = meta

    def self.parse(str):
        # TODO: Parse
        SemVer(...)

    def to_str(self):
        # TODO: add meta
        return "{}.{}.{}".format(self.major, self.minor, self.tiny)

    def __cmp__(self, other):
        # TODO: finish me
        return self.major <=> other.major
