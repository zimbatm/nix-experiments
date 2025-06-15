use anyhow::Result;
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::time::SystemTime;

use crate::config::Config;
use crate::environment::{Environment, EnvironmentType};
use crate::error::SandboxError;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CacheMetadata {
    pub created_at: DateTime<Utc>,
    pub environment_type: EnvironmentType,
    pub cache_key: String,
    pub nix_store_paths: Vec<String>,
    pub environment_vars: HashMap<String, String>,
}

impl CacheMetadata {
    pub fn new(
        environment_type: EnvironmentType,
        cache_key: String,
        nix_store_paths: Vec<String>,
        environment_vars: HashMap<String, String>,
    ) -> Self {
        Self {
            created_at: Utc::now(),
            environment_type,
            cache_key,
            nix_store_paths,
            environment_vars,
        }
    }

    pub fn is_valid(&self, current_cache_key: &str) -> bool {
        self.cache_key == current_cache_key
    }
}

#[derive(Debug)]
pub struct EnvironmentCache {
    config: Config,
}

impl EnvironmentCache {
    pub fn new(config: Config) -> Self {
        Self { config }
    }

    fn wrap_cache_error<T, E: Into<anyhow::Error>>(
        operation: &str,
        path: &Path,
        result: Result<T, E>,
    ) -> Result<T> {
        result.map_err(|e| {
            SandboxError::CacheError {
                operation: operation.to_string(),
                path: path.to_path_buf(),
                source: e.into(),
            }
            .into()
        })
    }

    pub fn get_cache_dir(&self, cache_key: &str) -> PathBuf {
        self.config.cache_dir.join(cache_key)
    }

    pub fn get_metadata_path(&self, cache_key: &str) -> PathBuf {
        self.get_cache_dir(cache_key).join("metadata.json")
    }

    pub fn exists(&self, cache_key: &str) -> bool {
        self.get_metadata_path(cache_key).exists()
    }

    pub fn load_metadata(&self, cache_key: &str) -> Result<CacheMetadata> {
        let metadata_path = self.get_metadata_path(cache_key);
        let content = Self::wrap_cache_error(
            "read metadata",
            &metadata_path,
            std::fs::read_to_string(&metadata_path),
        )?;

        let metadata: CacheMetadata = Self::wrap_cache_error(
            "parse metadata",
            &metadata_path,
            serde_json::from_str(&content),
        )?;

        Ok(metadata)
    }

    pub fn store_cache(
        &self,
        environment: &Environment,
        nix_store_paths: Vec<String>,
        environment_vars: HashMap<String, String>,
    ) -> Result<()> {
        let cache_key = environment.cache_key()?;
        let cache_dir = self.get_cache_dir(&cache_key);

        // Create cache directory
        Self::wrap_cache_error(
            "create cache directory",
            &cache_dir,
            std::fs::create_dir_all(&cache_dir),
        )?;

        // Create metadata
        let metadata = CacheMetadata::new(
            environment.env_type().clone(),
            cache_key,
            nix_store_paths,
            environment_vars,
        );

        // Write metadata to file
        let metadata_path = self.get_metadata_path(&metadata.cache_key);
        let metadata_content = Self::wrap_cache_error(
            "serialize metadata",
            &metadata_path,
            serde_json::to_string_pretty(&metadata),
        )?;

        Self::wrap_cache_error(
            "write metadata",
            &metadata_path,
            std::fs::write(&metadata_path, metadata_content),
        )?;

        Ok(())
    }

    pub fn get_cached_environment(
        &self,
        environment: &Environment,
    ) -> Result<Option<CacheMetadata>> {
        let cache_key = environment.cache_key()?;

        if !self.exists(&cache_key) {
            return Ok(None);
        }

        let metadata = self.load_metadata(&cache_key)?;

        // Validate cache is still current
        if !metadata.is_valid(&cache_key) {
            // Cache is stale, remove it
            self.remove_cache(&cache_key)?;
            return Ok(None);
        }

        // Verify all nix store paths still exist
        for store_path in &metadata.nix_store_paths {
            if !Path::new(store_path).exists() {
                // Cache is invalid, remove it
                self.remove_cache(&cache_key)?;
                return Ok(None);
            }
        }

        Ok(Some(metadata))
    }

    pub fn remove_cache(&self, cache_key: &str) -> Result<()> {
        let cache_dir = self.get_cache_dir(cache_key);
        if cache_dir.exists() {
            Self::wrap_cache_error(
                "remove cache",
                &cache_dir,
                std::fs::remove_dir_all(&cache_dir),
            )?;
        }
        Ok(())
    }

    pub fn cleanup_stale_caches(&self, max_age_days: u64) -> Result<()> {
        let cache_dir = &self.config.cache_dir;
        if !cache_dir.exists() {
            return Ok(());
        }

        let cutoff_time =
            SystemTime::now() - std::time::Duration::from_secs(max_age_days * 24 * 60 * 60);

        for entry in std::fs::read_dir(cache_dir)? {
            let entry = entry?;
            let path = entry.path();

            if !path.is_dir() {
                continue;
            }

            // Check if this is a cache directory (has metadata.json)
            let metadata_path = path.join("metadata.json");
            if !metadata_path.exists() {
                continue;
            }

            // Check metadata creation time
            let metadata = std::fs::metadata(&metadata_path)?;
            if let Ok(created) = metadata.created().or_else(|_| metadata.modified()) {
                if created < cutoff_time {
                    tracing::info!("Removing stale cache: {:?}", path);
                    std::fs::remove_dir_all(&path)?;
                }
            }
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::constants::environment;
    use std::collections::HashMap;
    use tempfile::TempDir;

    fn create_test_environment(temp_dir: &Path) -> Environment {
        let flake_path = temp_dir.join(environment::FLAKE_NIX);
        std::fs::write(&flake_path, "{}").unwrap();
        Environment::detect(temp_dir).unwrap()
    }

    fn create_test_config() -> Config {
        let temp_dir = TempDir::new().unwrap();
        let state_dir = temp_dir.path().to_path_buf();
        let sessions_dir = state_dir.join("sessions");
        let cache_dir = state_dir.join("cache");

        std::fs::create_dir_all(&sessions_dir).unwrap();
        std::fs::create_dir_all(&cache_dir).unwrap();

        Config {
            sessions_dir,
            cache_dir,
        }
    }

    #[test]
    fn test_cache_metadata_creation() {
        let env_vars = HashMap::from([("PATH".to_string(), "/nix/store/xyz/bin".to_string())]);
        let store_paths = vec!["/nix/store/abc123".to_string()];

        let metadata = CacheMetadata::new(
            EnvironmentType::Flake,
            "test_key".to_string(),
            store_paths.clone(),
            env_vars.clone(),
        );

        assert_eq!(metadata.environment_type, EnvironmentType::Flake);
        assert_eq!(metadata.cache_key, "test_key");
        assert_eq!(metadata.nix_store_paths, store_paths);
        assert_eq!(metadata.environment_vars, env_vars);
    }

    #[test]
    fn test_cache_metadata_validation() {
        let metadata = CacheMetadata::new(
            EnvironmentType::Flake,
            "test_key".to_string(),
            vec![],
            HashMap::new(),
        );

        assert!(metadata.is_valid("test_key"));
        assert!(!metadata.is_valid("different_key"));
    }

    #[test]
    fn test_cache_directory_creation() {
        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        let cache_dir = cache.get_cache_dir("test_key");
        let expected_path = cache.config.cache_dir.join("test_key");

        assert_eq!(cache_dir, expected_path);
    }

    #[test]
    fn test_cache_exists() {
        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        // Cache doesn't exist initially
        assert!(!cache.exists("nonexistent_key"));

        // Create metadata file
        let cache_dir = cache.get_cache_dir("test_key");
        std::fs::create_dir_all(&cache_dir).unwrap();
        std::fs::write(cache.get_metadata_path("test_key"), "{}").unwrap();

        assert!(cache.exists("test_key"));
    }

    #[test]
    fn test_store_and_load_cache() {
        let temp_dir = TempDir::new().unwrap();
        let environment = create_test_environment(temp_dir.path());

        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        let store_paths = vec!["/nix/store/test123".to_string()];
        let env_vars = HashMap::from([("PATH".to_string(), "/nix/store/test123/bin".to_string())]);

        // Store cache
        cache
            .store_cache(&environment, store_paths.clone(), env_vars.clone())
            .unwrap();

        // Load cache
        let cache_key = environment.cache_key().unwrap();
        let loaded_metadata = cache.load_metadata(&cache_key).unwrap();

        assert_eq!(loaded_metadata.environment_type, EnvironmentType::Flake);
        assert_eq!(loaded_metadata.nix_store_paths, store_paths);
        assert_eq!(loaded_metadata.environment_vars, env_vars);
    }

    #[test]
    fn test_get_cached_environment_valid() {
        let temp_dir = TempDir::new().unwrap();
        let environment = create_test_environment(temp_dir.path());

        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        // Store cache first
        let store_paths = vec![temp_dir.path().to_string_lossy().to_string()]; // Use temp dir as fake store path
        let env_vars = HashMap::new();
        cache
            .store_cache(&environment, store_paths, env_vars)
            .unwrap();

        // Get cached environment
        let result = cache.get_cached_environment(&environment).unwrap();
        assert!(result.is_some());

        let metadata = result.unwrap();
        assert_eq!(metadata.environment_type, EnvironmentType::Flake);
    }

    #[test]
    fn test_get_cached_environment_missing_store_path() {
        let temp_dir = TempDir::new().unwrap();
        let environment = create_test_environment(temp_dir.path());

        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        // Store cache with non-existent store path
        let store_paths = vec!["/nix/store/nonexistent123".to_string()];
        let env_vars = HashMap::new();
        cache
            .store_cache(&environment, store_paths, env_vars)
            .unwrap();

        // Get cached environment should return None and remove invalid cache
        let result = cache.get_cached_environment(&environment).unwrap();
        assert!(result.is_none());

        // Verify cache was removed
        let cache_key = environment.cache_key().unwrap();
        assert!(!cache.exists(&cache_key));
    }

    #[test]
    fn test_remove_cache() {
        let temp_dir = TempDir::new().unwrap();
        let environment = create_test_environment(temp_dir.path());

        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        // Store cache
        cache
            .store_cache(&environment, vec![], HashMap::new())
            .unwrap();

        let cache_key = environment.cache_key().unwrap();
        assert!(cache.exists(&cache_key));

        // Remove cache
        cache.remove_cache(&cache_key).unwrap();
        assert!(!cache.exists(&cache_key));
    }

    #[test]
    fn test_cache_key_mismatch_invalidates_cache() {
        let temp_dir = TempDir::new().unwrap();
        let flake_path = temp_dir.path().join(environment::FLAKE_NIX);
        std::fs::write(&flake_path, "{}").unwrap();

        let environment = Environment::detect(temp_dir.path()).unwrap();

        let config = create_test_config();
        let cache = EnvironmentCache::new(config);

        // Store cache
        cache
            .store_cache(&environment, vec![], HashMap::new())
            .unwrap();

        // Modify the flake file to change cache key
        std::thread::sleep(std::time::Duration::from_secs(1));
        std::fs::write(&flake_path, "{modified}").unwrap();

        let modified_environment = Environment::detect(temp_dir.path()).unwrap();

        // Getting cached environment should return None for modified environment
        let result = cache.get_cached_environment(&modified_environment).unwrap();
        assert!(result.is_none());
    }
}
