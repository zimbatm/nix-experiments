//! Progress output utilities for console display
//!
//! Provides formatted progress output for the installation process.

/// Installation step for progress display
#[derive(Debug, Clone, Copy)]
pub enum Step {
    Network,
    DiskDetection,
    Partition,
    Mount,
    FetchClosure,
    InstallBootloader,
    Complete,
}

impl Step {
    /// Get a human-readable description of this step
    pub fn description(&self) -> &'static str {
        match self {
            Step::Network => "Configuring network",
            Step::DiskDetection => "Detecting disks",
            Step::Partition => "Partitioning disk",
            Step::Mount => "Mounting filesystems",
            Step::FetchClosure => "Fetching NixOS system",
            Step::InstallBootloader => "Installing bootloader",
            Step::Complete => "Installation complete",
        }
    }

    /// Get the step number (1-based)
    pub fn number(&self) -> u8 {
        match self {
            Step::Network => 1,
            Step::DiskDetection => 2,
            Step::Partition => 3,
            Step::Mount => 4,
            Step::FetchClosure => 5,
            Step::InstallBootloader => 6,
            Step::Complete => 7,
        }
    }

    /// Total number of steps
    pub fn total() -> u8 {
        7
    }
}

/// Output text to console
pub fn output(msg: &str) {
    println!("{}", msg);
}

/// Output a step progress message with formatting
pub fn step(s: Step, detail: &str) {
    const CYAN: &str = "\x1b[36m";
    const GREEN: &str = "\x1b[32m";
    const RESET: &str = "\x1b[0m";

    println!(
        "{CYAN}[{}/{}]{RESET} {GREEN}{}{RESET}: {}",
        s.number(),
        Step::total(),
        s.description(),
        detail
    );
}

/// Output an error message
#[allow(dead_code)]
pub fn error(msg: &str) {
    const RED: &str = "\x1b[31m";
    const RESET: &str = "\x1b[0m";
    println!("{RED}ERROR:{RESET} {}", msg);
}

/// Output a warning message
#[allow(dead_code)]
pub fn warn(msg: &str) {
    const YELLOW: &str = "\x1b[33m";
    const RESET: &str = "\x1b[0m";
    println!("{YELLOW}WARN:{RESET} {}", msg);
}
