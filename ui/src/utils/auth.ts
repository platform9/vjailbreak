export const getUserInfo = () => {
    // Extract user info from OAuth2 proxy headers (set by ingress)
    // These would be available in response headers from API calls
    return {
      user: localStorage.getItem('user-email'),
      groups: JSON.parse(localStorage.getItem('user-groups') || '[]'),
    }
  }
  
  export const hasPermission = (resource: string, verb: string): boolean => {
    const groups = JSON.parse(localStorage.getItem('user-groups') || '[]')
    
    // Admin has all permissions
    if (groups.includes('vjailbreak-admins')) {
      return true
    }
    
    // Operator can create/update migrations but not delete credentials
    if (groups.includes('vjailbreak-operators')) {
      const readOnlyResources = ['openstackcreds', 'vmwarecreds', 'networkmappings', 'storagemappings']
      if (readOnlyResources.includes(resource) && ['delete', 'update'].includes(verb)) {
        return false
      }
      return !['delete'].includes(verb)
    }
    
    // Viewer can only read
    if (groups.includes('vjailbreak-viewers')) {
      return verb === 'get' || verb === 'list'
    }
    
    return false
  }
  
  export const checkAuth = () => {
    // Check if user is authenticated by trying to fetch user info
    // OAuth2 proxy will redirect to login if not authenticated
    return fetch('/oauth2/userinfo', {
      credentials: 'include'
    })
      .then(res => res.json())
      .then(data => {
        localStorage.setItem('user-email', data.email)
        localStorage.setItem('user-groups', JSON.stringify(data.groups || []))
        return true
      })
      .catch(() => {
        // Not authenticated, will be redirected by OAuth2 proxy
        return false
      })
  }

  export const logout = () => {
    // Clear local storage
    localStorage.removeItem('user-email')
    localStorage.removeItem('user-groups')
    
    // Redirect to OAuth2 proxy sign out endpoint
    window.location.href = '/oauth2/sign_out'
  }