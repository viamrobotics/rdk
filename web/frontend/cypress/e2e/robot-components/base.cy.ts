describe('base', () => {
  it('should be interactive', () => {
    cy.visit('/');

    // Open base
    cy.contains('h2', 'test_base').should('exist').click();

    // Activate and deactivate keyboard
    cy.get('[aria-label="Enable keyboard"]').should('exist').click();

    // Select camera
    cy.get('[aria-label="Refresh frequency for test_camera"]').click();
  });
});

export {};
