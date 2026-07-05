import { Injectable } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { MatSnackBar } from '@angular/material/snack-bar';

@Injectable({ providedIn: 'root' })
export class ErrorService {
  constructor(private readonly snackBar: MatSnackBar) {}

  handleError(error: HttpErrorResponse): void {
    const message = this.getErrorMessage(error);
    this.showError(message);
  }

  showError(message: string): void {
    this.snackBar.open(message, 'Dismiss', {
      duration: 5000,
      panelClass: ['error-snackbar'],
      horizontalPosition: 'center',
      verticalPosition: 'bottom',
    });
  }

  private getErrorMessage(error: HttpErrorResponse): string {
    if (error.status === 0) {
      return 'Unable to connect to server. Please check your internet connection.';
    }

    if (error.error?.message) {
      return error.error.message;
    }

    switch (error.status) {
      case 403:
        return 'Access denied. You do not have permission to perform this action.';
      case 404:
        return 'The requested resource was not found.';
      case 500:
        return 'An unexpected server error occurred. Please try again later.';
      default:
        return `An error occurred (${error.status}). Please try again.`;
    }
  }
}
