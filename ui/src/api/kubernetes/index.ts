/**
 * Kubernetes API module for pod operations
 * 
 * This module provides centralized functions for making API calls to Kubernetes pod resources.
 * It specifically focuses on pod listing and log streaming operations.
 */

export { fetchPods, streamPodLogs, type Pod, type PodListResponse } from "./pods"
