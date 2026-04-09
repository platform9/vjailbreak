---
paths:
  - "ui/**/*.{ts,tsx,js,jsx}"
  - "ui/**/*.css"
---

# UI Development Rules

Rules for developing the vJailbreak React/TypeScript frontend.

## Tech Stack

- **Framework**: React with TypeScript
- **Build Tool**: Vite
- **UI Library**: Material-UI (MUI)
- **State Management**: React hooks and context
- **Icons**: Lucide React
- **Styling**: TailwindCSS + MUI theming

## Development Setup

### Environment Variables
Required environment variables for development:
```bash
export VITE_API_HOST=<backend-host>
export VITE_API_TOKEN=<auth-token>
```

### Development Server
```bash
cd ui/
yarn install
yarn dev       # Runs on http://localhost:3000
```

### Production Build
```bash
cd ui/
yarn build
```

## Code Style

### TypeScript
- Use strict TypeScript - no `any` types unless absolutely necessary
- Define proper interfaces for all data structures
- Use type inference where possible
- Export types for reusable components

### Component Structure
- Functional components with hooks (no class components)
- One component per file
- Co-locate component-specific styles and tests
- Use named exports for components

### Naming Conventions
- Components: PascalCase (e.g., `MigrationForm.tsx`)
- Hooks: camelCase with `use` prefix (e.g., `useMigrationStatus.ts`)
- Utilities: camelCase (e.g., `formatDate.ts`)
- Constants: UPPER_SNAKE_CASE

## MUI Best Practices

### Component Usage
- Use MUI components consistently across the app
- Follow MUI theming for colors, spacing, and typography
- Customize theme in theme configuration file
- Use MUI's responsive utilities for layout

### Styling
- Prefer MUI's `sx` prop for component-specific styles
- Use theme values for consistency (spacing, colors, breakpoints)
- Avoid inline styles unless dynamic
- Use TailwindCSS utilities for common patterns

## API Integration

### Backend Communication
- All API calls should go through centralized API client
- Handle loading states consistently
- Implement proper error handling and user feedback
- Use TypeScript interfaces for API request/response types

### Error Handling
- Display user-friendly error messages
- Log errors to console for debugging
- Implement retry logic for transient failures
- Show appropriate fallback UI for error states

## State Management

### Component State
- Use `useState` for local component state
- Use `useEffect` for side effects
- Use `useCallback` and `useMemo` for performance optimization
- Avoid prop drilling - use context for deeply nested state

### Form Handling
- Validate user input before submission
- Provide clear validation error messages
- Disable submit buttons during API calls
- Reset forms after successful submission

## Testing

### Unit Tests
- Write tests for utility functions
- Test component rendering and interactions
- Mock API calls in tests
- Use React Testing Library patterns

### E2E Tests
- Cypress tests in `ui/cypress/`
- Test critical user flows (migration creation, status monitoring)
- Test error scenarios and edge cases

## Build and Deployment

### Docker Image
```bash
# From repo root
make ui
```

### Environment Configuration
- Use environment variables for configuration
- Never hardcode API endpoints or credentials
- Support different environments (dev, staging, prod)

## Performance

### Optimization
- Lazy load routes and heavy components
- Optimize images and assets
- Use code splitting for large bundles
- Monitor bundle size

### Best Practices
- Avoid unnecessary re-renders
- Use React.memo for expensive components
- Debounce user input for search/filter
- Implement pagination for large data sets

## Accessibility

- Use semantic HTML elements
- Provide alt text for images
- Ensure keyboard navigation works
- Use ARIA labels where appropriate
- Test with screen readers

## Common Patterns

### Loading States
```typescript
const [loading, setLoading] = useState(false);
const [error, setError] = useState<string | null>(null);
const [data, setData] = useState<DataType | null>(null);
```

### API Calls
```typescript
useEffect(() => {
  const fetchData = async () => {
    setLoading(true);
    try {
      const result = await apiClient.getData();
      setData(result);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };
  fetchData();
}, []);
```

## Debugging

### Browser DevTools
- Use React DevTools for component inspection
- Check Network tab for API calls
- Monitor Console for errors and warnings
- Use Redux DevTools if using Redux

### Common Issues
- **API connectivity**: Verify `VITE_API_HOST` and `VITE_API_TOKEN`
- **CORS errors**: Check backend CORS configuration
- **Build failures**: Clear node_modules and reinstall
- **Hot reload issues**: Restart dev server

## Documentation

### Component Documentation
- Add JSDoc comments for complex components
- Document props with TypeScript interfaces
- Include usage examples for reusable components
- Keep README updated with setup instructions
