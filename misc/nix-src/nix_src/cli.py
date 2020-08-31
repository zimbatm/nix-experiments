import argparse
import json
import re
import subprocess
import sys
import urllib.request

REPO_URI_RE = re.compile("^(?P<owner>[\w-]+)/(?P<repo>[\w-]+)(?:@(?P<rev>.*))?$")

def parse_repo_uri(str):
    result = REPO_URI_RE.match(str)
    if result is None:
        return None
    return dict(
        owner=result.group('owner'),
        repo=result.group('repo'),
        rev=result.group('rev'),
    )

def http_get(url):
    with urllib.request.urlopen(url) as response:
       return response.read()

def get_branch_sha1(repo_info, branch):
    url_template="https://api.github.com/repos/{owner}/{repo}/git/refs/heads/{branch}"
    url = url_template.format(owner=repo_info["owner"],
                              repo=repo_info["repo"],
                              branch=branch)
    data = json.loads(http_get(url))
    return data["object"]["sha"]

def nix_prefetch_github(repo_info):
    url="https://github.com/{owner}/{repo}/archive/{rev}.tar.gz".format(**repo_info)
    ret = subprocess.check_output(["nix-prefetch-url", url])
    return ret.decode("utf-8").strip()

def dump_json(repo_info):
    return json.dumps(repo_info, sort_keys=True, indent=2, separators=(',', ': '))

def dump_nix(repo_info):
    return '''
fetchFromGitHub {
  owner  = "%s";
  repo   = "%s";
  rev    = "%s";
  sha256 = "%s";
}
    ''' % (repo_info["owner"], repo_info["repo"], repo_info["rev"], repo_info["sha256"])

class Actions:
    def dump(_, args):
        repo_info = parse_repo_uri(args.repo)

        branch = None

        # convert branch into sha1 if rev is missing
        if repo_info["rev"] is None:
            sha1 = get_branch_sha1(repo_info, args.branch)
            repo_info["rev"] = sha1

        # find sha256 of the target
        repo_info["sha256"] = nix_prefetch_github(repo_info)

        dumper = dump_nix
        if args.format == "json":
            dumper = dump_json

        out = sys.stdout
        if args.out != "-":
            out = open(args.out, "w")

        out.write(dumper(repo_info))

def main():
    parser = argparse.ArgumentParser(
        description='utility to handle nixpkgs source pinning',
        epilog='woot',
    )
    parser.add_argument('-r', '--repo',
                        metavar='owner/repo[@rev]',
                        default='NixOS/nixpkgs',
                        help='picks which repo to use')
    parser.add_argument('-b', '--branch',
                        default="master",
                        help='lookup the rev on the given branch if missing')
    parser.add_argument('-f', '--format',
                        choices=['json', 'nix'],
                        default='nix',
                        help='sets the output format')
    parser.add_argument('-o', '--out',
                        default='-',
                        help='sets the output target. "-" represents stdout')
    parser.add_argument('action',
                        choices=['init', 'dump', 'update'],
                        default='dump',
                        help='select which action to execute')

    args = parser.parse_args()

    # Dispatch the action
    actions = Actions()
    action = getattr(actions, args.action)
    action(args)
