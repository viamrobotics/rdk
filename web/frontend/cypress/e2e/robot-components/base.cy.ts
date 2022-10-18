describe('should interact with base', () => {
  it('passes', () => {
    cy.visit('/');

    // Open base
    cy.contains('h2', 'test_base').should('exist').click();

    // Activate Keyboard
    cy.get('[aria-label="Keyboard Disabled"]').should('exist').click();
    // Deactivate keyboard
    cy.get('[aria-label="Keyboard Enabled"]').should('exist').click();

    cy.get('[aria-label="Select Cameras"]').find('[aria-disabled="false"]').click().type('test_camera{enter}');

    cy.wait(5000);

  });
});

export {};
