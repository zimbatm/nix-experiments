# Chronixpkgs

**Build reactive tools for nixpkgs without fighting GitHub's API limits.**

Chronixpkgs provides a persistent event stream for nixpkgs, enabling developers to create tools that react to repository changes in real-time.

## The Problem

Want to build a bot that responds to PRs? A dashboard showing package updates? A notification system for security fixes?

GitHub's Events API makes this harder than it should be:
- Events disappear after 90 days
- Limited to 300 events per page
- Requires constant polling and state management

## The Solution

Chronixpkgs maintains a complete, queryable event history:
- **Real-time streaming** via Server-Sent Events
- **Full history** - no 90-day limit
- **Simple queries** - filter by event type, time range
- **Always available** - no rate limits for reads

## Quick Example

Watch for new pull requests in real-time:

```javascript
const eventSource = new EventSource('https://events.nixos.org/events?type=PullRequestEvent');

eventSource.onmessage = (e) => {
  const event = JSON.parse(e.data);
  if (event.payload.action === 'opened') {
    console.log(`New PR: ${event.payload.pull_request.title}`);
    // Trigger your automation here
  }
};
```

## What Can You Build?

- **PR Bots** - Auto-label, assign reviewers, check conventions
- **Update Trackers** - Monitor package updates and breaking changes  
- **Security Scanners** - Alert on vulnerability fixes
- **Analytics Dashboards** - Visualize contributor activity
- **Build Triggers** - React to specific package changes
- **Notification Systems** - Custom alerts for maintainers

## Getting Started

### For Tool Developers

Check out the [Quick Start Guide](docs/QUICK_START.md) to start building.

Full API documentation: [docs/API.md](docs/API.md)

### For Operators

Run your own instance:

```bash
# Quick start with Nix
nix run github:zimbatm/nix-experiments?dir=chronixpkgs

# Or see deployment guide
cat docs/DEPLOYMENT.md
```

## Documentation

- [Quick Start](docs/QUICK_START.md) - Start using the API in 5 minutes
- [API Reference](docs/API.md) - Complete endpoint documentation
- [Architecture](docs/ARCHITECTURE.md) - How it works under the hood
- [Configuration](docs/CONFIGURATION.md) - All configuration options
- [Development](docs/DEVELOPMENT.md) - Contributing and local setup
- [Deployment](docs/DEPLOYMENT.md) - Running your own instance

## Contributing

We welcome contributions! See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for guidelines.

## License

MIT - see [LICENSE](LICENSE) for details
