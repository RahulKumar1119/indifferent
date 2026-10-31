#!/usr/bin/env python3
"""A stand-in for the aws, docker, git and sleep commands, for testing
deploy/setup-storyvideo.sh without an AWS account.

It keeps a "world" (the AWS resources that exist) in the JSON file named by
$STUB_WORLD. Read calls answer from it, in the shape and text the real CLI gives
for the queries the script uses. Calls that change something change the world, so
a second run of the script sees what the first one made. Every call is appended to
$STUB_CALLS as one JSON line, with `mutating` set for the ones that change
something. A call the stub does not know fails loudly (exit 255) and is recorded
as `unhandled`: an unexpected command is a test failure, not a silent success.

It is a model of the CLI as the script uses it, not a copy of AWS. The one thing
it evaluates itself is IAM (a small matcher for Action and Resource), so the
policy checks in the tests are checks against this matcher, not against AWS.
"""
import fnmatch
import json
import os
import sys

# Unset when this file is imported (a test uses the IAM matcher), set when it runs.
WORLD = os.environ.get("STUB_WORLD")
CALLS = os.environ.get("STUB_CALLS")

# Operations that change something. Everything else must be read-only.
MUTATING = {
    ("ssm", "put-parameter"),
    ("iam", "create-role"), ("iam", "attach-role-policy"), ("iam", "put-role-policy"),
    ("s3api", "put-bucket-lifecycle-configuration"),
    ("ecr", "create-repository"), ("ecr", "put-lifecycle-policy"),
    ("logs", "create-log-group"), ("logs", "put-retention-policy"),
    ("ecs", "register-task-definition"),
    ("lambda", "update-function-configuration"),
    ("dynamodb", "create-table"), ("dynamodb", "update-time-to-live"), ("dynamodb", "put-item"),
    ("stepfunctions", "create-state-machine"), ("stepfunctions", "update-state-machine"),
    ("docker", "login"), ("docker", "build"), ("docker", "push"),
}


def load():
    with open(WORLD) as f:
        return json.load(f)


def save(w):
    with open(WORLD, "w") as f:
        json.dump(w, f, indent=1)


def record(tool, argv, op, mutating, files, **extra):
    with open(CALLS, "a") as f:
        f.write(json.dumps({"tool": tool, "argv": argv, "op": op, "mutating": mutating, "files": files, **extra}) + "\n")


def flag(argv, name, default=None):
    """The value after --name, or default."""
    for i, a in enumerate(argv):
        if a == name and i + 1 < len(argv):
            return argv[i + 1]
    return default


def read_file_arg(v):
    """A CLI parameter value: file://path is read, anything else is as given."""
    if v and v.startswith("file://"):
        with open(v[len("file://"):]) as f:
            return f.read()
    return v


def err(code, message, rc=254):
    sys.stderr.write("\nAn error occurred (%s) when calling the operation: %s\n" % (code, message))
    sys.exit(rc)


def out(text):
    sys.stdout.write(str(text) + "\n")


def match_any(patterns, value, fold=False):
    if isinstance(patterns, str):
        patterns = [patterns]
    for p in patterns:
        if fold:
            if fnmatch.fnmatchcase(value.lower(), p.lower()):
                return True
        elif fnmatch.fnmatchcase(value, p):
            return True
    return False


def allowed(policies, action, resource):
    """Allow when some Allow statement matches both. No Deny statements are used."""
    for doc in policies:
        for st in doc.get("Statement", []):
            if st.get("Effect") == "Allow" and match_any(st["Action"], action, fold=True) and match_any(st["Resource"], resource):
                return True
    return False


def dynamodb_stock_cache(w, op, rest, q):
    """The stock-cache table (deploy/setup-stock-cache.sh, flush-stockmedia-cache.sh).
    World key "stock_cache": None, or {"status", "keys", "billing", "ttl": {"status",
    "attr"}, "items": {pk: item}}. Answers in the text the CLI prints for the queries
    used. Returns True when it handled the call; False for anything it does not
    model, which the caller then records as unhandled (a test failure)."""
    t = w.get("stock_cache")
    if op == "create-table":
        if t is not None:
            err("ResourceInUseException", "Table already exists")
        attrs = rest[rest.index("--attribute-definitions") + 1]
        keys = rest[rest.index("--key-schema") + 1]
        assert attrs == "AttributeName=PK,AttributeType=S", attrs
        assert keys == "AttributeName=PK,KeyType=HASH", keys
        w["stock_cache"] = {"status": "ACTIVE", "keys": [["PK", "HASH"]], "billing": flag(rest, "--billing-mode"),
                            "ttl": {"status": "DISABLED", "attr": None}, "items": {}}
        save(w)
        out("CREATING")
        return True
    if t is None:
        # describe-table is how a script asks whether the table exists; the others cannot work without it.
        if op in ("describe-table", "wait", "describe-time-to-live", "update-time-to-live", "get-item", "put-item"):
            err("ResourceNotFoundException", "Requested resource not found")
        return False
    if op == "wait":
        out("")
        return True
    if op == "describe-table" and q == "Table.TableStatus":
        out(t["status"])
        return True
    if op == "describe-table" and q == "Table.KeySchema[].[AttributeName,KeyType]":
        out("\n".join("\t".join(k) for k in t["keys"]))
        return True
    if op == "describe-time-to-live" and q == "TimeToLiveDescription.[TimeToLiveStatus,AttributeName]":
        out("%s\t%s" % (t["ttl"]["status"], t["ttl"]["attr"] or "None"))
        return True
    if op == "update-time-to-live":
        spec = dict(kv.split("=") for kv in flag(rest, "--time-to-live-specification").split(","))
        assert spec["Enabled"] == "true", spec
        t["ttl"] = {"status": "ENABLED", "attr": spec["AttributeName"]}
        save(w)
        out(spec["AttributeName"])
        return True
    if op == "get-item" and q == "Item.epoch.S":
        pk = json.loads(flag(rest, "--key"))["PK"]["S"]
        item = t["items"].get(pk)
        w.setdefault("stock_cache_reads", []).append({"pk": pk, "consistent": "--consistent-read" in rest})
        save(w)
        out(item["epoch"]["S"] if item and "epoch" in item else "None")
        return True
    if op == "put-item":
        item = json.loads(flag(rest, "--item"))
        t["items"][item["PK"]["S"]] = item
        save(w)
        out("")
        return True
    return False


def aws(argv):
    svc, op = argv[0], argv[1]
    rest = argv[2:]
    w = load()
    mutating = (svc, op) in MUTATING
    files = {}
    # Files that carry a secret (a stock key, the API Lambda's whole environment)
    # are not copied into the call log, so the log can be searched for secrets.
    # Their location and permissions are recorded instead: that they sit in a
    # private place is part of what the tests check.
    secret_file_ops = {("ssm", "put-parameter"), ("lambda", "update-function-configuration")}
    fileinfo = {}
    for a in rest:
        if a.startswith("file://") and os.path.isfile(a[7:]):
            if (svc, op) in secret_file_ops:
                st, dst = os.stat(a[7:]), os.stat(os.path.dirname(a[7:]))
                fileinfo[a] = {"path": a[7:], "mode": st.st_mode & 0o777, "dirmode": dst.st_mode & 0o777}
            else:
                files[a] = open(a[7:]).read()
    record("aws", argv, "%s %s" % (svc, op), mutating, files, fileinfo=fileinfo)
    q = flag(rest, "--query")
    acct = w["account"]

    if (svc, op) == ("sts", "get-caller-identity"):
        return out(acct)

    if svc == "s3api":
        if op == "head-bucket":
            return out("") if w["bucket"] else err("404", "Not Found")
        if op == "get-bucket-lifecycle-configuration":
            if w["lifecycle"] is None:
                return err("NoSuchLifecycleConfiguration", "The lifecycle configuration does not exist")
            return out(json.dumps(w["lifecycle"]))
        if op == "put-bucket-lifecycle-configuration":
            cfg = json.loads(read_file_arg(flag(rest, "--lifecycle-configuration")))
            if set(cfg) != {"Rules"}:
                sys.stderr.write("Unknown parameter in LifecycleConfiguration: %s\n" % sorted(set(cfg) - {"Rules"}))
                sys.exit(252)
            size = flag(rest, "--transition-default-minimum-object-size", "all_storage_classes_128K")
            w["lifecycle"] = {"TransitionDefaultMinimumObjectSize": size, "Rules": cfg["Rules"]}
            return save(w)

    if svc == "dynamodb" and flag(rest, "--table-name") == "indifferent-fun-stock-cache":
        if dynamodb_stock_cache(w, op, rest, q):
            return
        # not modelled: fall through to the unhandled path at the end of aws()

    if (svc, op) == ("dynamodb", "describe-table"):
        return out("ACTIVE") if w["table"] else err("ResourceNotFoundException", "Requested resource not found")

    if (svc, op) == ("ecs", "describe-clusters"):
        c = w["cluster"]
        if c is None:
            return out("MISSING") if q.endswith("status") else out("None")
        return out(c["status"]) if q.endswith("status") else out("\t".join(c["providers"]))

    if (svc, op) == ("ec2", "describe-security-groups"):
        return out(w["sg"] or "None")

    if (svc, op) == ("ec2", "describe-subnets"):
        wanted = []
        if "--subnet-ids" in rest:  # one or more ids, up to the next option
            for a in rest[rest.index("--subnet-ids") + 1:]:
                if a.startswith("--"):
                    break
                wanted.append(a)
        have = [s for s in wanted if s in w["subnets"]]
        if len(have) != len(wanted):
            return err("InvalidSubnetID.NotFound", "The subnet ID does not exist")
        return out(len(have))

    if svc == "lambda":
        fn = flag(rest, "--function-name")
        if op == "wait":
            return out("")
        if fn == "indifferent-fun-api":
            api = w["api"]
            if not api["exists"]:
                return err("ResourceNotFoundException", "Function not found: arn:aws:lambda:ap-south-1:%s:function:%s" % (acct, fn))

            def config():
                cfg = {"FunctionName": fn, "RevisionId": api["revision"], "LastUpdateStatus": api.get("status", "Successful"), "Runtime": "provided.al2023"}
                if api["env"] is not None:
                    cfg["Environment"] = {"Variables": api["env"]}
                return cfg

            if op == "get-function-configuration":
                text = json.dumps(config())
                if api.get("race"):  # somebody else changes the function right after we read it
                    api["revision"] = "rev-raced"
                    save(w)
                return out(text)
            if op == "update-function-configuration":
                rev = flag(rest, "--revision-id")
                if rev is not None and rev != api["revision"]:
                    return err("PreconditionFailedException", "The Revision Id provided does not match the latest Revision Id. Call the GetFunction/GetAlias API to retrieve the latest Revision Id")
                env = json.loads(read_file_arg(flag(rest, "--environment")))
                if set(env) != {"Variables"} or not all(isinstance(v, str) for v in env["Variables"].values()):
                    return err("InvalidParameterValueException", "bad environment")
                api["env"] = env["Variables"]
                api["revision"] = "rev-%d" % (api.get("updates", 0) + 2)  # any new value, so a stale read is refused
                api["updates"] = api.get("updates", 0) + 1
                save(w)
                # The real CLI prints the whole configuration, environment included,
                # unless --query narrows it. The script must narrow it.
                if q == "LastModified":
                    return out("2026-10-31T12:00:00.000+0000")
                return out(json.dumps(config()))
        n = w["notifier"]
        if op == "get-function-configuration":
            return out("indifferent-fun-notifier") if n["exists"] else err("ResourceNotFoundException", "Function not found")
        if op == "invoke":
            text = "send failed: jobId is required for kind story" if n["knows_story"] else 'send failed: unknown kind "story"'
            with open(rest[-1], "w") as f:
                json.dump({"errorMessage": text, "errorType": "SendFailed"}, f)
            return out("")

    if svc == "ssm":
        name = flag(rest, "--name")
        if op == "get-parameter":
            return out(w["ssm"][name]) if name in w["ssm"] else err("ParameterNotFound", "Parameter not found")
        if op == "put-parameter":
            value = read_file_arg(flag(rest, "--value"))
            w["ssm"][name] = flag(rest, "--type")
            w.setdefault("ssm_values", {})[name] = value
            save(w)
            return out(1)

    if svc == "iam":
        if op == "get-role":
            return out("{}") if flag(rest, "--role-name") in w["roles"] else err("NoSuchEntity", "role not found")
        if op == "create-role":
            name = flag(rest, "--role-name")
            if name in w["roles"]:
                return err("EntityAlreadyExists", "exists")
            json.loads(read_file_arg(flag(rest, "--assume-role-policy-document")))
            w["roles"][name] = {"attached": [], "policies": {}, "base": []}
            save(w)
            return out("arn:aws:iam::%s:role/%s" % (acct, name))
        role = flag(rest, "--role-name")
        if op == "list-attached-role-policies":
            for p in w["roles"].get(role, {}).get("attached", []):
                return out(p)
            return out("None")
        if op == "attach-role-policy":
            w["roles"][role]["attached"].append(flag(rest, "--policy-arn").rsplit("/", 1)[-1])
            return save(w)
        if op == "get-role-policy":
            pol = w["roles"].get(role, {}).get("policies", {}).get(flag(rest, "--policy-name"))
            return out(json.dumps(pol)) if pol else err("NoSuchEntity", "policy not found")
        if op == "put-role-policy":
            w["roles"][role]["policies"][flag(rest, "--policy-name")] = json.loads(read_file_arg(flag(rest, "--policy-document")))
            return save(w)
        if op in ("simulate-principal-policy", "simulate-custom-policy"):
            if w.get("simulator") == "denied":
                return err("AccessDenied", "User is not authorized to perform: iam:%s" % op, rc=254)
            req = json.loads(read_file_arg(flag(rest, "--cli-input-json")))
            policies = [json.loads(p) for p in req.get("PolicyInputList", [])]
            if op == "simulate-principal-policy":
                r = w["roles"][req["PolicySourceArn"].rsplit("/", 1)[-1]]
                policies += r["base"] + list(r["policies"].values())
            results = []
            for action in req["ActionNames"]:
                for res in req["ResourceArns"]:
                    results.append({"EvalActionName": action, "EvalResourceName": res,
                                    "EvalDecision": "allowed" if allowed(policies, action, res) else "implicitDeny"})
            return out(json.dumps({"EvaluationResults": results}))

    if svc == "ecr":
        repo = w["ecr"]
        if op == "describe-repositories":
            return out("{}") if repo["exists"] else err("RepositoryNotFoundException", "not found")
        if op == "create-repository":
            repo.update(exists=True, mutable=flag(rest, "--image-tag-mutability"), scan=flag(rest, "--image-scanning-configuration"))
            save(w)
            return out("123456789012.dkr.ecr.ap-south-1.amazonaws.com/" + flag(rest, "--repository-name"))
        if op == "describe-images":
            tag = flag(rest, "--image-ids").split("=", 1)[1]
            return out("{}") if repo["exists"] and tag in repo["images"] else err("ImageNotFoundException", "not found")
        if op == "get-lifecycle-policy":
            return out(repo["policy"]) if repo["policy"] else err("LifecyclePolicyNotFoundException", "not found")
        if op == "put-lifecycle-policy":
            repo["policy"] = read_file_arg(flag(rest, "--lifecycle-policy-text"))
            return save(w)
        if op == "get-login-password":
            return out("stub-token")

    if svc == "logs":
        lg = w["logs"]
        if op == "describe-log-groups":
            if not lg["exists"]:
                return out("None")
            return out(lg["retention"] if "retentionInDays" in q and lg["retention"] else ("None" if "retentionInDays" in q else flag(rest, "--log-group-name-prefix")))
        if op == "create-log-group":
            lg["exists"] = True
            return save(w)
        if op == "put-retention-policy":
            lg["retention"] = flag(rest, "--retention-in-days")
            return save(w)

    if svc == "ecs":
        if op == "describe-task-definition":
            return out(json.dumps({"taskDefinition": w["taskdefs"][-1]})) if w["taskdefs"] else err("ClientException", "Unable to describe task definition.")
        if op == "register-task-definition":
            fam = flag(rest, "--family")
            containers = json.loads(read_file_arg(flag(rest, "--container-definitions")))
            for c in containers:  # AWS fills in empty lists and defaults
                c.update(portMappings=[], mountPoints=[], volumesFrom=[], systemControls=[])
            plat = dict(p.split("=") for p in flag(rest, "--runtime-platform").split(","))
            td = {
                "family": fam, "revision": len(w["taskdefs"]) + 1, "status": "ACTIVE",
                "taskDefinitionArn": "arn:aws:ecs:ap-south-1:%s:task-definition/%s:%d" % (acct, fam, len(w["taskdefs"]) + 1),
                "cpu": flag(rest, "--cpu"), "memory": flag(rest, "--memory"),
                "executionRoleArn": flag(rest, "--execution-role-arn"), "taskRoleArn": flag(rest, "--task-role-arn"),
                "networkMode": flag(rest, "--network-mode"), "requiresCompatibilities": ["FARGATE"],
                "runtimePlatform": {"cpuArchitecture": plat["cpuArchitecture"], "operatingSystemFamily": plat["operatingSystemFamily"]},
                "containerDefinitions": containers,
            }
            w["taskdefs"].append(td)
            save(w)
            return out(td["taskDefinitionArn"])

    if svc == "stepfunctions":
        if op == "validate-state-machine-definition":
            d = json.loads(read_file_arg(flag(rest, "--definition")))
            assert "States" in d
            if q == "result":
                return out(w["validator"])
            return out(json.dumps([{"severity": "ERROR", "code": "STUB", "message": "rejected by the stub"}]))
        if op == "describe-state-machine":
            return out(w["sm"]) if w["sm"] is not None else err("StateMachineDoesNotExist", "State Machine Does Not Exist")
        if op in ("create-state-machine", "update-state-machine"):
            w["sm"] = read_file_arg(flag(rest, "--definition"))
            w["sm_role"] = flag(rest, "--role-arn")
            save(w)
            return out("arn:aws:states:ap-south-1:%s:stateMachine:%s" % (acct, flag(rest, "--name", "x")))

    # Anything else is a mistake in the script or a gap in the stub.
    record("aws", argv, "%s %s" % (svc, op), mutating, files, unhandled=True)
    sys.stderr.write("stub: unhandled aws %s\n" % " ".join(argv))
    sys.exit(255)


def docker(argv):
    op = argv[0]
    w = load()
    stdin_used = False
    if op == "login":
        sys.stdin.read()
        stdin_used = True
    record("docker", argv, "docker " + op, op in ("login", "build", "push"), {}, stdin=stdin_used)
    if op == "login":
        return
    if op == "build":
        dockerfile, ctx = flag(argv, "-f"), argv[-1]
        if not (os.path.isfile(dockerfile) and os.path.isdir(ctx)):
            sys.stderr.write("stub: docker build needs a real Dockerfile (%s) and context (%s)\n" % (dockerfile, ctx))
            sys.exit(1)
        w.setdefault("built", []).append(flag(argv, "-t"))
        return save(w)
    if op == "push":
        image = argv[1]
        repo, tag = image.rsplit(":", 1)
        if not w["ecr"]["exists"] or image not in w.get("built", []):
            sys.stderr.write("stub: cannot push %s (no repository, or not built)\n" % image)
            sys.exit(1)
        w["ecr"]["images"].append(tag)
        return save(w)
    record("docker", argv, "docker " + op, False, {}, unhandled=True)
    sys.exit(255)


def git(argv):
    w = load()["git"]
    record("git", argv, "git " + argv[0], False, {})
    if argv[0] == "-C" and argv[2:4] == ["rev-parse", "--short=12"]:
        return out(w["sha"])
    if argv[0] == "-C" and argv[2:4] == ["status", "--porcelain"]:
        for line in w["dirty"]:
            out(line)
        return
    record("git", argv, "git " + argv[0], False, {}, unhandled=True)
    sys.stderr.write("stub: unhandled git %s\n" % " ".join(argv))
    sys.exit(255)


def main():
    tool, argv = sys.argv[1], sys.argv[2:]
    if tool == "sleep":
        return record("sleep", argv, "sleep", False, {})
    {"aws": aws, "docker": docker, "git": git}[tool](argv)


if __name__ == "__main__":
    main()
